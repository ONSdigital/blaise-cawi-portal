package webserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/blaise"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type InstrumentController struct {
	Auth       authenticate.AuthInterface
	JWTCrypto  authenticate.JWTCryptoInterface
	CatiUrl    string
	HttpClient *http.Client
}

func (instrumentController *InstrumentController) AddRoutes(httpRouter *gin.Engine) {
	instrumentRouter := httpRouter.Group("/:instrumentName")
	instrumentRouter.Use(instrumentController.Auth.Authenticated)
	{
		instrumentRouter.GET("/", func(context *gin.Context) {
			instrumentController.openCase(context)
		})
		// Example path /dst2101a/resources/js/jskdjasjdlkasjld.js
		// instrumentName = dst2101a
		// path = resources
		// resource = /js/jskdjasjdlkasjld.js
		instrumentRouter.GET("/:path/*resource", func(context *gin.Context) {
			instrumentController.proxyGet(context)
		})
		// Above root would only match /dst2101a/resources/*
		// We have to add this to additonally match /dst2101a/resources
		instrumentRouter.GET("/:path", func(context *gin.Context) {
			instrumentController.proxyGet(context)
		})

		instrumentRouter.POST("/*path", func(context *gin.Context) {
			instrumentController.proxyPost(context)
		})
	}
}

func (instrumentController *InstrumentController) instrumentAuth(context *gin.Context) (*authenticate.UACClaims, error) {
	session := sessions.Default(context)
	jwtToken := session.Get(authenticate.JWT_TOKEN_KEY)
	uacClaim, err := instrumentController.JWTCrypto.DecryptJWT(jwtToken)
	if err != nil {
		authenticate.NotAuthWithError(context, authenticate.INTERNAL_SERVER_ERR)
		return nil, err
	}
	instrumentName := context.Param("instrumentName")
	if !uacClaim.AuthenticatedForInstrument(instrumentName) {
		authenticate.Forbidden(context)
		return nil, fmt.Errorf("Forbidden")
	}
	return uacClaim, nil
}

func (instrumentController *InstrumentController) openCase(context *gin.Context) {
	uacClaim, err := instrumentController.instrumentAuth(context)
	if err != nil {
		return
	}
	resp, err := http.PostForm(
		fmt.Sprintf("%s/%s/default.aspx", instrumentController.CatiUrl, uacClaim.UacInfo.InstrumentName),
		blaise.CasePayload(uacClaim.UacInfo.CaseID).Form(),
	)
	if err != nil {
		InternalServerError(context)
		return
	}

	if resp.StatusCode != http.StatusOK {
		InternalServerError(context)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		InternalServerError(context)
		return
	}

	defer resp.Body.Close()
	context.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

func (instrumentController *InstrumentController) proxyGet(context *gin.Context) {
	path := context.Param("path")
	resource := context.Param("resource")

	uacClaim, err := instrumentController.instrumentAuth(context)
	if err != nil {
		return
	}

	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/%s/%s%s", instrumentController.CatiUrl, uacClaim.UacInfo.InstrumentName, path, resource), nil)
	addHeaders(req, context)
	resp, err := instrumentController.HttpClient.Do(req)
	if err != nil {
		InternalServerError(context)
		return
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error proxying GET '%s/%s/%s%s' status code: '%v'\n",
			instrumentController.CatiUrl, uacClaim.UacInfo.InstrumentName, path, resource, resp.StatusCode)
		InternalServerError(context)
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		InternalServerError(context)
		return
	}
	defer resp.Body.Close()
	context.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

func (instrumentController *InstrumentController) proxyPost(context *gin.Context) {
	uacClaim, err := instrumentController.instrumentAuth(context)
	if err != nil {
		return
	}
	path := context.Param("path")
	if path == "/api/application/start_interview" {
		instrumentController.proxyStartInterview(context, path, uacClaim)
	} else {
		requestBody, err := ioutil.ReadAll(context.Request.Body)
		if err != nil {
			InternalServerError(context)
			return
		}
		instrumentController.proxyPoster(context,
			fmt.Sprintf("%s/%s%s", instrumentController.CatiUrl, uacClaim.UacInfo.InstrumentName, path),
			bytes.NewBuffer(requestBody),
		)
	}
}

func (instrumentController *InstrumentController) proxyStartInterview(context *gin.Context, path string, uacClaim *authenticate.UACClaims) {
	var startInterview blaise.StartInterview
	startInterviewBody, err := ioutil.ReadAll(context.Request.Body)
	if err != nil {
		fmt.Println(err)
		InternalServerError(context)
		return
	}

	err = json.Unmarshal(startInterviewBody, &startInterview)
	if err != nil {
		fmt.Println(err)
		InternalServerError(context)
		return
	}

	if !uacClaim.AuthenticatedForCase(startInterview.RuntimeParameters.KeyValue) {
		authenticate.Forbidden(context)
		return
	}
	instrumentController.proxyPoster(context,
		fmt.Sprintf("%s/%s%s", instrumentController.CatiUrl, uacClaim.UacInfo.InstrumentName, path),
		bytes.NewBuffer(startInterviewBody),
	)
}

func (instrumentController *InstrumentController) proxyPoster(context *gin.Context, url string, requestBody io.Reader) {
	contentType := context.Request.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	req, _ := http.NewRequest("POST", url, requestBody)
	addHeaders(req, context)
	req.Header.Set("Content-Type", contentType)
	resp, err := instrumentController.HttpClient.Do(req)
	defer context.Request.Body.Close()
	if err != nil {
		InternalServerError(context)
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		InternalServerError(context)
		return
	}
	defer resp.Body.Close()
	context.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

func addHeaders(req *http.Request, context *gin.Context) {
	for header, value := range context.Request.Header {
		header = strings.ToLower(header)
		if header == "content-encoding" || header == "content-length" ||
			header == "transfer-encoding" || header == "connection" {
			continue
		}
		req.Header.Set(header, strings.Join(value, ", s"))
	}
}
