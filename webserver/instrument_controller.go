package webserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	// "strings"

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
	instrumentRouter.Use(instrumentController.Auth.AuthenticatedWithUacAndPostcode)
	{
		instrumentRouter.GET("/", instrumentController.openCase)
		// Example path /dst2101a/resources/js/jskdjasjdlkasjld.js
		// instrumentName = dst2101a
		// path = resources
		// resource = /js/jskdjasjdlkasjld.js
		instrumentRouter.Any("/:path/*resource", instrumentController.proxyWithInstrumentAuth)
		// Above root would only match /dst2101a/resources/*
		// We have to add this to additonally match /dst2101a/resources
		instrumentRouter.Any("/:path", instrumentController.proxyWithInstrumentAuth)
	}

	httpRouter.GET("/:instrumentName/logout", instrumentController.logoutEndpoint)
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

func (instrumentController *InstrumentController) proxyWithInstrumentAuth(context *gin.Context) {
	uacClaim, err := instrumentController.instrumentAuth(context)
	if err != nil {
		return
	}
	path := context.Param("path")
	resource := context.Param("resource")
	if fmt.Sprintf("/%s%s", path, resource) == "/api/application/start_interview" {
		if instrumentController.startInterviewAuth(context, path, uacClaim) {
			return
		}
	}
	instrumentController.proxy(context, uacClaim)
}

func (instrumentController *InstrumentController) startInterviewAuth(context *gin.Context, path string, uacClaim *authenticate.UACClaims) bool {
	var startInterview blaise.StartInterview
	var buffer bytes.Buffer
	startInterviewTee := io.TeeReader(context.Request.Body, &buffer)
	startInterviewBody, err := ioutil.ReadAll(startInterviewTee)
	if err != nil {
		fmt.Println(err)
		InternalServerError(context)
		return true
	}

	err = json.Unmarshal(startInterviewBody, &startInterview)
	if err != nil {
		fmt.Println(err)
		InternalServerError(context)
		return true
	}

	if !uacClaim.AuthenticatedForCase(startInterview.RuntimeParameters.KeyValue) {
		authenticate.Forbidden(context)
		return true
	}
	context.Request.Body = ioutil.NopCloser(&buffer)
	return false
}

func (instrumentController *InstrumentController) proxy(context *gin.Context, uacClaim *authenticate.UACClaims) {
	path := context.Param("path")
	resource := context.Param("resource")

	remote, err := url.Parse(instrumentController.CatiUrl)
	if err != nil {
		InternalServerError(context)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)
	proxy.Director = func(req *http.Request) {
		req.Host = remote.Hostname()
		req.URL.Scheme = remote.Scheme
		req.URL.Host = remote.Host
		req.URL.Path = fmt.Sprintf("%s/%s%s", uacClaim.InstrumentName, path, resource)
		log.Printf("Request Host: %s\n", req.Host)
		log.Printf("Request URL: %s://%s%s", req.URL.Scheme, req.URL.Host, req.URL.Path)
	}
	proxy.ServeHTTP(context.Writer, context.Request)
}

func (instrumentController *InstrumentController) logoutEndpoint(context *gin.Context) {
	session := sessions.Default(context)

	instrumentController.Auth.Logout(context, session)
}
