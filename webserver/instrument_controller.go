package webserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/blaise"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type InstrumentController struct {
	Auth       authenticate.AuthInterface
	JWTCrypto  authenticate.JWTCryptoInterface
	Logger     *zap.Logger
	CatiUrl    string
	HttpClient *http.Client
	Debug      bool
}

func (instrumentController *InstrumentController) AddRoutes(httpRouter *gin.Engine) {
	instrumentRouter := httpRouter.Group("/:instrumentName")
	instrumentRouter.Use(instrumentController.Auth.AuthenticatedWithUac)
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
		instrumentController.Logger.Error("Error decrypting JWT", zap.Error(err))
		instrumentController.Auth.NotAuthWithError(context, authenticate.INTERNAL_SERVER_ERR)
		return nil, err
	}
	instrumentName := context.Param("instrumentName")
	if !uacClaim.AuthenticatedForInstrument(instrumentName) {
		instrumentController.Logger.Info("Not authenticated for instrument",
			append(uacClaim.LogFields(), zap.String("InstrumentName", instrumentName))...)
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
		instrumentController.Logger.Error("Error launching blaise study", append(uacClaim.LogFields(), zap.Error(err))...)
		InternalServerError(context)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		instrumentController.Logger.Error("Error launching blaise study, cannot read response body",
			append(uacClaim.LogFields(), zap.Error(err))...)
		InternalServerError(context)
		return
	}

	if resp.StatusCode != http.StatusOK {
		instrumentController.Logger.Error("Error launching blaise study, invalid status code",
			append(uacClaim.LogFields(),
				zap.Int("RespStatusCode", resp.StatusCode),
				zap.ByteString("RespBody", body),
			)...)
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
	if isStartInterviewUrl(path, resource) {
		if instrumentController.startInterviewAuth(context, uacClaim) {
			return
		}
	}
	instrumentController.proxy(context, uacClaim)
}

func (instrumentController *InstrumentController) startInterviewAuth(context *gin.Context, uacClaim *authenticate.UACClaims) bool {
	var startInterview blaise.StartInterview
	var buffer bytes.Buffer
	startInterviewTee := io.TeeReader(context.Request.Body, &buffer)
	startInterviewBody, err := ioutil.ReadAll(startInterviewTee)
	if err != nil {
		instrumentController.Logger.Error("Error reading start interview request body",
			append(uacClaim.LogFields(), zap.Error(err))...)
		InternalServerError(context)
		return true
	}

	err = json.Unmarshal(startInterviewBody, &startInterview)
	if err != nil {
		instrumentController.Logger.Error("Error JSON decoding start interview request",
			append(uacClaim.LogFields(), zap.Error(err))...)
		InternalServerError(context)
		return true
	}

	if !uacClaim.AuthenticatedForCase(startInterview.RuntimeParameters.KeyValue) {
		instrumentController.Logger.Info("Not authenticated to start interview for case",
			append(uacClaim.LogFields(), zap.String("CaseID", startInterview.RuntimeParameters.KeyValue))...)
		authenticate.Forbidden(context)
		return true
	}
	context.Request.Body = ioutil.NopCloser(&buffer)
	return false
}

func (instrumentController *InstrumentController) proxy(context *gin.Context, uacClaim *authenticate.UACClaims) {
	remote, err := url.Parse(instrumentController.CatiUrl)
	if err != nil {
		instrumentController.Logger.Error("Could not parse url for proxying", zap.String("URL", instrumentController.CatiUrl))
		InternalServerError(context)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)

	if instrumentController.Debug {
		proxy.Transport = &debugTransport{Logger: instrumentController.Logger}
	}

	proxy.ServeHTTP(context.Writer, context.Request)
}

func (instrumentController *InstrumentController) logoutEndpoint(context *gin.Context) {
	session := sessions.Default(context)

	instrumentController.Auth.Logout(context, session)
}

func isStartInterviewUrl(path, resource string) bool {
	return fmt.Sprintf("/%s%s", path, resource) == "/api/application/start_interview"
}

type debugTransport struct {
	Logger *zap.Logger
}

func (debugTransport *debugTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	b, err := httputil.DumpRequestOut(r, false)
	if err != nil {
		return nil, err
	}
	debugTransport.Logger.Debug("Proxy round trip debug", zap.ByteString("RequestDump", b))
	return http.DefaultTransport.RoundTrip(r)
}
