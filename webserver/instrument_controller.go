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

func (instrumentController *InstrumentController) openCase(context *gin.Context) {
	session := sessions.Default(context)
	jwtToken := session.Get(authenticate.JWT_TOKEN_KEY)
	uacClaim, err := instrumentController.Auth.DecryptJWT(jwtToken)
	if err != nil {
		authenticate.NotAuthWithError(context, authenticate.INTERNAL_SERVER_ERR)
		return
	}
	instrumentName := context.Param("instrumentName")
	if !uacClaim.AuthenticatedForInstrument(instrumentName) {
		authenticate.Forbidden(context)
		return
	}

	resp, err := http.PostForm(
		fmt.Sprintf("%s/%s/default.aspx", instrumentController.CatiUrl, uacClaim.UacInfo.InstrumentName),
		blaise.CasePayload(uacClaim.CaseID).Form(),
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
	instrumentName := context.Param("instrumentName")
	path := context.Param("path")
	resource := context.Param("resource")

	session := sessions.Default(context)
	jwtToken := session.Get(authenticate.JWT_TOKEN_KEY)
	uacClaim, err := instrumentController.Auth.DecryptJWT(jwtToken)
	if err != nil {
		authenticate.NotAuthWithError(context, authenticate.INTERNAL_SERVER_ERR)
		return
	}
	if !uacClaim.AuthenticatedForInstrument(instrumentName) {
		authenticate.Forbidden(context)
		return
	}

	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/%s/%s%s", instrumentController.CatiUrl, instrumentName, path, resource), nil)
	addHeaders(req, context)
	resp, err := instrumentController.HttpClient.Do(req)
	if err != nil {
		InternalServerError(context)
		return
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error proxying GET '%s/%s/%s%s' status code: '%v'\n",
			instrumentController.CatiUrl, instrumentName, path, resource, resp.StatusCode)
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
	instrumentName := context.Param("instrumentName")
	path := context.Param("path")
	if path == "/api/application/start_interview" {
		instrumentController.proxyStartInterview(context, instrumentName, path)
	} else {
		requestBody, err := ioutil.ReadAll(context.Request.Body)
		if err != nil {
			InternalServerError(context)
			return
		}
		instrumentController.proxyPoster(context, fmt.Sprintf("%s/%s%s", instrumentController.CatiUrl, instrumentName, path), bytes.NewBuffer(requestBody))
	}
	return
}

func (instrumentController *InstrumentController) proxyStartInterview(context *gin.Context, instrumentName, path string) {
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

	session := sessions.Default(context)
	jwtToken := session.Get(authenticate.JWT_TOKEN_KEY)
	uacClaim, err := instrumentController.Auth.DecryptJWT(jwtToken)
	if err != nil {
		authenticate.NotAuthWithError(context, authenticate.INTERNAL_SERVER_ERR)
		return
	}
	if !uacClaim.AuthenticatedForCase(startInterview.RuntimeParameters.KeyValue) {
		authenticate.Forbidden(context)
		return
	}
	instrumentController.proxyPoster(context, fmt.Sprintf("%s/%s%s", instrumentController.CatiUrl, instrumentName, path), bytes.NewBuffer(startInterviewBody))
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
	return
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

// func instrumentGroup(httpRouter *gin.Engine, auth *Auth, controller *Controller) {
// 	instrumentRouter := httpRouter.Group("/:instrumentName")
// 	{
// 		instrumentRouter.GET("/*path", func(context *gin.Context) {
// 			auth.Required(context)
// 			instrumentName := context.Param("instrumentName")
// 			path := context.Param("path")
// 			if !auth.AuthenticatedForInstrument(context, instrumentName) {
// 				context.AbortWithStatus(http.StatusForbidden)
// 				return
// 			}
// 			if path == "/" {
// 				controller.openCase(context, auth)
// 				return
// 			} else {
// 				controller.proxyGet(context, instrumentName, path)
// 			}
// 			return
// 		})

// 		instrumentRouter.POST("/*path", func(context *gin.Context) {
// 			auth.Required(context)
// 			instrumentName := context.Param("instrumentName")
// 			path := context.Param("path")
// 			if !auth.AuthenticatedForInstrument(context, instrumentName) {
// 				context.AbortWithStatus(http.StatusForbidden)
// 			}
// 			controller.proxyPost(context, instrumentName, path, auth)
// 			return
// 		})
// 	}
