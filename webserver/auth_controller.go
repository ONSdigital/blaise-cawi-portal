package webserver

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/csrf"
	"github.com/ONSdigital/blaise-cawi-portal/languagemanager"
	"github.com/ONSdigital/blaise-cawi-portal/sessionkeys"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AuthController struct {
	Auth            authenticate.AuthInterface
	Logger          *zap.Logger
	CSRFManager     csrf.CSRFManager
	LanguageManager languagemanager.LanguageManagerInterface
}

func (authController *AuthController) AddRoutes(httpRouter *gin.Engine) {
	authGroup := httpRouter.Group("/auth")
	authGroup.Use(authController.CSRFManager.Middleware())
	{
		authGroup.GET("/login", authController.LoginEndpoint)
		authGroup.POST("/login", authController.PostLoginEndpoint)
		authGroup.GET("/logout", authController.LogoutEndpoint)
		authGroup.GET("/logged-in", authController.LoggedInEndpoint)
		authGroup.GET("/timed-out", authController.TimedOutEndpoint)
	}
}

func (authController *AuthController) LoginEndpoint(context *gin.Context) {
	hasSession, claim := authController.Auth.HasSession(context)
	if hasSession {
		context.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/%s/", claim.UACInfo.InstrumentName))
		return
	}

	requestedLang := languagemanager.GetLangFromQuery(context)
	currentlyWelsh := authController.LanguageManager.IsWelsh(context)
	if requestedLang == "en" && currentlyWelsh {
		authController.LanguageManager.SetWelsh(context, false)
	}
	if requestedLang == "cy" && !currentlyWelsh {
		authController.LanguageManager.SetWelsh(context, true)
	}

	context.HTML(http.StatusOK, "login.tmpl", gin.H{
		"uac16":      authController.Auth.IsUAC16(),
		"csrf_token": authController.CSRFManager.GetToken(context),
		"welsh":      authController.LanguageManager.IsWelsh(context),
	})
}

func (authController *AuthController) PostLoginEndpoint(context *gin.Context) {
	session := sessions.DefaultMany(context, sessionkeys.UserSessionName)

	uac := strings.ReplaceAll(context.PostForm("uac"), " ", "")
	if uac != "" {
		isUAC16 := authController.Auth.IsUAC16()
		expectedLen, lengthDesc, welshDesc := 12, "12-digit", "12 o nodau"
		if isUAC16 {
			expectedLen, lengthDesc, welshDesc = 16, "16-character", "16 o nodau"
		}
		if len(uac) != expectedLen {
			var errMsg string
			if authController.LanguageManager.IsWelsh(context) {
				errMsg = fmt.Sprintf(authenticate.INVALID_LENGTH_ERR["welsh"], welshDesc)
			} else {
				errMsg = fmt.Sprintf(authenticate.INVALID_LENGTH_ERR["english"], lengthDesc)
			}
			context.HTML(http.StatusForbidden, "login.tmpl", gin.H{
				"error":      errMsg,
				"uac16":      isUAC16,
				"csrf_token": authController.CSRFManager.GetToken(context),
				"welsh":      authController.LanguageManager.IsWelsh(context),
			})
			context.Abort()
			return
		}
	}

	authController.Auth.Login(context, session)
}

func (authController *AuthController) LogoutEndpoint(context *gin.Context) {
	session := sessions.DefaultMany(context, sessionkeys.UserSessionName)

	authController.Auth.Logout(context, session)
}

func (authController *AuthController) LoggedInEndpoint(context *gin.Context) {
	authenticated, _ := authController.Auth.HasSession(context)
	if !authenticated {
		context.Status(http.StatusUnauthorized)
		return
	}
	context.Status(http.StatusOK)
}

func (authController *AuthController) TimedOutEndpoint(context *gin.Context) {
	session := sessions.DefaultMany(context, sessionkeys.UserSessionName)

	timeout := authenticate.DefaultAuthTimeout
	if timeoutValue := session.Get(authenticate.SESSION_TIMEOUT_KEY); timeoutValue != nil {
		if sessionTimeout, ok := timeoutValue.(int); ok && sessionTimeout > 0 {
			timeout = sessionTimeout
		}
	}

	context.HTML(http.StatusOK, "timeout.tmpl", gin.H{
		"timeout": timeout,
		"welsh":   authController.LanguageManager.IsWelsh(context),
	})
}
