package webserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/blaise"
	"github.com/ONSdigital/blaise-cawi-portal/languagemanager"
	"github.com/ONSdigital/blaise-cawi-portal/utils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/net/html"
)

// TODO(Blaise 5.16 upgrade): remove support for the legacy default.aspx launch route.
// In newer Blaise questionnaires, default.aspx is removed and the launch request should
// hit the instrument root URL (served via MVC views such as _Layout.cshtml).
var launchPaths = []string{"default.aspx", ""}

var errForbiddenInstrumentAccess = errors.New("forbidden instrument access")

type InstrumentController struct {
	Auth            authenticate.AuthInterface
	JWTCrypto       authenticate.JWTCryptoInterface
	Logger          *zap.Logger
	CatiURL         string
	HttpClient      *http.Client
	Debug           bool
	LanguageManager languagemanager.LanguageManagerInterface
}

func (instrumentController *InstrumentController) logger() *zap.Logger {
	if instrumentController.Logger != nil {
		return instrumentController.Logger
	}
	return zap.L()
}

func (instrumentController *InstrumentController) AddRoutes(httpRouter *gin.Engine) {
	instrumentRouter := httpRouter.Group("/:instrumentName")
	instrumentRouter.Use(instrumentController.Auth.AuthenticatedWithUAC)
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
	session := sessions.DefaultMany(context, "user_session")
	jwtToken := session.Get(authenticate.JWT_TOKEN_KEY)
	uacClaim, err := instrumentController.JWTCrypto.DecryptJWT(jwtToken)
	if err != nil {
		instrumentController.logger().Error("Error decrypting JWT", zap.Error(err))
		instrumentController.Auth.NotAuthWithError(context, instrumentController.LanguageManager.LanguageError(authenticate.INTERNAL_SERVER_ERR, context))
		return nil, fmt.Errorf("failed to decrypt JWT for instrument auth: %w", err)
	}
	instrumentName := context.Param("instrumentName")
	sanitizedInstrumentName := utils.SanitizeLogInput(instrumentName)
	if !uacClaim.AuthenticatedForInstrument(instrumentName) {
		instrumentController.logger().Info("Not authenticated for instrument",
			append(uacClaim.LogFields(), zap.String("InstrumentName", sanitizedInstrumentName))...)
		authenticate.Forbidden(context, instrumentController.LanguageManager.IsWelsh(context))
		return nil, fmt.Errorf("authentication failed for instrument %q: %w", sanitizedInstrumentName, errForbiddenInstrumentAccess)
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
	resp, err := instrumentController.launchCase(context, uacClaim)
	if err != nil {
		instrumentController.logger().Error("Error launching blaise study", append(uacClaim.LogFields(), zap.Error(err))...)
		InternalServerError(context, instrumentController.LanguageManager.IsWelsh(context))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		instrumentController.logger().Error("Error launching blaise study, cannot read response body",
			append(uacClaim.LogFields(), zap.Error(err))...)
		InternalServerError(context, instrumentController.LanguageManager.IsWelsh(context))
		return
	}

	if resp.StatusCode != http.StatusOK {
		instrumentController.logger().Error("Error launching blaise study, invalid status code",
			append(uacClaim.LogFields(),
				zap.Int("RespStatusCode", resp.StatusCode),
				zap.Int("RespBodyBytes", len(body)),
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
				instrumentController.logger().Error("Error rendering HTML",
					append(uacClaim.LogFields(), zap.Error(err))...)
			}
		} else {
			instrumentController.logger().Error("Error injecting check-session script",
				append(uacClaim.LogFields(), zap.Error(err))...)
		}
	}

	context.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

func (instrumentController *InstrumentController) launchCase(context *gin.Context, uacClaim *authenticate.UACClaims) (*http.Response, error) {
	form := blaise.CasePayload(uacClaim.UACInfo.CaseID, instrumentController.LanguageManager.IsWelsh(context)).Form()

	for i, path := range launchPaths {
		launchURL := fmt.Sprintf("%s/%s/", instrumentController.CatiURL, uacClaim.UACInfo.InstrumentName)
		if path != "" {
			launchURL = fmt.Sprintf("%s/%s/%s", instrumentController.CatiURL, uacClaim.UACInfo.InstrumentName, path)
		}

		resp, err := instrumentController.HttpClient.PostForm(
			launchURL,
			form,
		)
		if err != nil {
			return nil, err
		}

		// TODO(Blaise 5.16 upgrade): delete this 404 fallback branch when default.aspx is retired.
		if resp.StatusCode == http.StatusNotFound && i < len(launchPaths)-1 {
			resp.Body.Close()
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("failed to launch case for instrument %s", uacClaim.UACInfo.InstrumentName)
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
		instrumentController.logger().Error("Error reading start interview request body",
			append(uacClaim.LogFields(), zap.Error(err))...)
		InternalServerError(context, instrumentController.LanguageManager.IsWelsh(context))
		return true
	}

	err = json.Unmarshal(startInterviewBody, &startInterview)
	if err != nil {
		instrumentController.logger().Error("Error JSON decoding start interview request",
			append(uacClaim.LogFields(), zap.Error(err))...)
		InternalServerError(context, instrumentController.LanguageManager.IsWelsh(context))
		return true
	}

	if !uacClaim.AuthenticatedForCase(startInterview.RuntimeParameters.KeyValue) {
		instrumentController.logger().Info("Not authenticated to start interview for case",
				append(uacClaim.LogFields(), zap.String("CaseIDFingerprint", authenticate.CaseIDFingerprint(startInterview.RuntimeParameters.KeyValue)))...)
		authenticate.Forbidden(context, instrumentController.LanguageManager.IsWelsh(context))
		return true
	}
	context.Request.Body = io.NopCloser(&buffer)
	return false
}

func (instrumentController *InstrumentController) proxy(context *gin.Context, uacClaim *authenticate.UACClaims) {
	remote, err := url.Parse(instrumentController.CatiURL)
	if err != nil {
		instrumentController.logger().Error("Could not parse url for proxying", zap.String("URL", instrumentController.CatiURL))
		InternalServerError(context, instrumentController.LanguageManager.IsWelsh(context))
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)

	if instrumentController.Debug {
		proxy.Transport = &debugTransport{Logger: instrumentController.logger()}
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

func InjectScript(body []byte) (*html.Node, error) {
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
