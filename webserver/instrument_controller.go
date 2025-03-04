package webserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"unicode"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/blaise"
	"github.com/ONSdigital/blaise-cawi-portal/languagemanager"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/net/html"
)

type InstrumentController struct {
	Auth            authenticate.AuthInterface
	JWTCrypto       authenticate.JWTCryptoInterface
	Logger          *zap.Logger
	CatiUrl         string
	HttpClient      *http.Client
	Debug           bool
	LanguageManager languagemanager.LanguageManagerInterface
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

func sanitizeLogInput(input string) string {
	escapedInput := html.EscapeString(input)
	escapedInput = strings.ReplaceAll(escapedInput, "\n", "")
	escapedInput = strings.ReplaceAll(escapedInput, "\r", "")
	escapedInput = strings.ReplaceAll(escapedInput, "\t", "")
	return strings.Map(func(r rune) rune {
		if !unicode.IsPrint(r) {
			return -1
		}
		return r
	}, escapedInput)
}

func (instrumentController *InstrumentController) instrumentAuth(context *gin.Context) (*authenticate.UACClaims, error) {
	session := sessions.DefaultMany(context, "user_session")
	jwtToken := session.Get(authenticate.JWT_TOKEN_KEY)
	uacClaim, err := instrumentController.JWTCrypto.DecryptJWT(jwtToken)
	if err != nil {
		instrumentController.Logger.Error("Error decrypting JWT", zap.Error(err))
		instrumentController.Auth.NotAuthWithError(context, instrumentController.LanguageManager.LanguageError(authenticate.INTERNAL_SERVER_ERR, context))
		return nil, err
	}
	instrumentName := context.Param("instrumentName")
	sanitizedInstrumentName := sanitizeLogInput(instrumentName)
	if !uacClaim.AuthenticatedForInstrument(instrumentName) {
		instrumentController.Logger.Info("Not authenticated for instrument",
			append(uacClaim.LogFields(), zap.String("InstrumentName", sanitizedInstrumentName))...)
		authenticate.Forbidden(context, instrumentController.LanguageManager.IsWelsh(context))
		return nil, fmt.Errorf("Forbidden")
	}
	if isAPICall(context) {
		instrumentController.Auth.RefreshToken(context, session, uacClaim)
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
		blaise.CasePayload(uacClaim.UacInfo.CaseID, instrumentController.LanguageManager.IsWelsh(context)).Form(),
	)
	if err != nil {
		instrumentController.Logger.Error("Error launching blaise study", append(uacClaim.LogFields(), zap.Error(err))...)
		InternalServerError(context, instrumentController.LanguageManager.IsWelsh(context))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		instrumentController.Logger.Error("Error launching blaise study, cannot read response body",
			append(uacClaim.LogFields(), zap.Error(err))...)
		InternalServerError(context, instrumentController.LanguageManager.IsWelsh(context))
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		instrumentController.Logger.Error("Error launching blaise study, invalid status code",
			append(uacClaim.LogFields(),
				zap.Int("RespStatusCode", resp.StatusCode),
				zap.ByteString("RespBody", body),
			)...)
		InternalServerError(context, instrumentController.LanguageManager.IsWelsh(context))
		return
	}

	if getContentType(resp) == "text/html" {
		var buf bytes.Buffer
		injectedBody, err := InjectScript(body)
		if err == nil {
			err = html.Render(&buf, injectedBody)
			if err == nil {
				body = buf.Bytes()
			} else {
				instrumentController.Logger.Error("Error rendering HTML",
					append(uacClaim.LogFields(), zap.Error(err))...)
			}
		} else {
			instrumentController.Logger.Error("Error injecting check-session script",
				append(uacClaim.LogFields(), zap.Error(err))...)
		}
	}

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
	startInterviewBody, err := io.ReadAll(startInterviewTee)
	if err != nil {
		instrumentController.Logger.Error("Error reading start interview request body",
			append(uacClaim.LogFields(), zap.Error(err))...)
		InternalServerError(context, instrumentController.LanguageManager.IsWelsh(context))
		return true
	}

	err = json.Unmarshal(startInterviewBody, &startInterview)
	if err != nil {
		instrumentController.Logger.Error("Error JSON decoding start interview request",
			append(uacClaim.LogFields(), zap.Error(err))...)
		InternalServerError(context, instrumentController.LanguageManager.IsWelsh(context))
		return true
	}

	if !uacClaim.AuthenticatedForCase(startInterview.RuntimeParameters.KeyValue) {
		sanitizedCaseID := sanitizeLogInput(startInterview.RuntimeParameters.KeyValue)
		instrumentController.Logger.Info("Not authenticated to start interview for case",
			append(uacClaim.LogFields(), zap.String("CaseID", sanitizedCaseID))...)
		authenticate.Forbidden(context, instrumentController.LanguageManager.IsWelsh(context))
		return true
	}
	context.Request.Body = io.NopCloser(&buffer)
	return false
}

func (instrumentController *InstrumentController) proxy(context *gin.Context, uacClaim *authenticate.UACClaims) {
	remote, err := url.Parse(instrumentController.CatiUrl)
	if err != nil {
		instrumentController.Logger.Error("Could not parse url for proxying", zap.String("URL", instrumentController.CatiUrl))
		InternalServerError(context, instrumentController.LanguageManager.IsWelsh(context))
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)

	if instrumentController.Debug {
		proxy.Transport = &debugTransport{Logger: instrumentController.Logger}
	}

	proxy.ServeHTTP(context.Writer, context.Request)
}

func (instrumentController *InstrumentController) logoutEndpoint(context *gin.Context) {
	session := sessions.DefaultMany(context, "user_session")
	instrumentController.Auth.Logout(context, session)
}

func isStartInterviewUrl(path, resource string) bool {
	return fmt.Sprintf("/%s%s", path, resource) == "/api/application/start_interview"
}

func isAPICall(context *gin.Context) bool {
	path := context.Param("path")
	resource := context.Param("resource")
	return path == "api" || resource == "api" ||
		strings.Contains(path, "/api/") || strings.Contains(resource, "/api/")
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

func InjectScript(body []byte) (*html.Node, error) {
	fmt.Println("Inject Script")
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var crawler func(*html.Node)
	crawler = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "body" {
			scriptNode := &html.Node{
				Type: html.ElementNode,
				Data: "script",
				Attr: []html.Attribute{
					{Key: "src", Val: "/assets/js/check-session.js"},
				},
			}
			node.AppendChild(scriptNode)
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			crawler(child)
		}
	}
	crawler(doc)
	return doc, nil
}

func getContentType(resp *http.Response) string {
	contentType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	return contentType
}
