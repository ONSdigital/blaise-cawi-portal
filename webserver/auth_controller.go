package webserver

import (
	"fmt"
	"net/http"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/languagemanager"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	csrf "github.com/srbry/gin-csrf"
	"go.uber.org/zap"
)

type AuthController struct {
	Auth            authenticate.AuthInterface
	Logger          *zap.Logger
	UacKind         string
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
		context.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/%s/", claim.UacInfo.InstrumentName))
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
		"uac16":      authController.isUac16(),
		"csrf_token": authController.CSRFManager.GetToken(context),
		"welsh":      authController.LanguageManager.IsWelsh(context),
	})
}

func (authController *AuthController) PostLoginEndpoint(context *gin.Context) {
	session := sessions.DefaultMany(context, "user_session")

	authController.Auth.Login(context, session)
}

func (authController *AuthController) LogoutEndpoint(context *gin.Context) {
	session := sessions.DefaultMany(context, "user_session")

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
	session := sessions.DefaultMany(context, "user_session")

	timeout := session.Get(authenticate.SESSION_TIMEOUT_KEY)
	if timeout != nil {
		timeout = timeout.(int)
	}
	if timeout == nil || timeout == 0 {
		timeout = authenticate.DefaultAuthTimeout
	}

	context.HTML(http.StatusOK, "timeout.tmpl", gin.H{
		"timeout": timeout,
		"welsh":   authController.LanguageManager.IsWelsh(context),
	})
}

func (authController *AuthController) isUac16() bool {
	return authController.UacKind == "uac16"
}
