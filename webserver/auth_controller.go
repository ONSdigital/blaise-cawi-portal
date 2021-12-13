package webserver

import (
	"fmt"
	"net/http"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/utils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	csrf "github.com/utrack/gin-csrf"
	"go.uber.org/zap"
)

type AuthController struct {
	Auth       authenticate.AuthInterface
	Logger     *zap.Logger
	CSRFSecret string
	UacKind    string
}

func (authController *AuthController) AddRoutes(httpRouter *gin.Engine) {
	authGroup := httpRouter.Group("/auth")
	authGroup.Use(csrf.Middleware(csrf.Options{
		Secret: authController.CSRFSecret,
		ErrorFunc: func(context *gin.Context) {
			authController.Logger.Info("CSRF mismatch", utils.GetRequestSource(context)...)
			context.HTML(http.StatusForbidden, "login.tmpl", gin.H{
				"uac16":      authController.isUac16(),
				"info":       "Request timed out, please try again",
				"csrf_token": csrf.GetToken(context),
			})
			context.Abort()
		},
	}))
	{
		authGroup.GET("/login", authController.LoginEndpoint)
		authGroup.POST("/login", authController.PostLoginEndpoint)
		authGroup.GET("/logout", authController.LogoutEndpoint)
		authGroup.GET("/logged-in", authController.LoggedInEndpoint)
		authGroup.GET("/timed-out", authController.TimedOutEndpoint)
	}
}

func (authController *AuthController) LoginEndpoint(context *gin.Context) {
	context.Set("csrfSecret", authController.CSRFSecret)
	hasSession, claim := authController.Auth.HasSession(context)
	if hasSession {
		context.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/%s/", claim.UacInfo.InstrumentName))
		return
	}

	context.HTML(http.StatusOK, "login.tmpl", gin.H{
		"uac16":      authController.isUac16(),
		"csrf_token": csrf.GetToken(context),
	})
}

func (authController *AuthController) PostLoginEndpoint(context *gin.Context) {
	session := sessions.Default(context)

	authController.Auth.Login(context, session)
}

func (authController *AuthController) LogoutEndpoint(context *gin.Context) {
	session := sessions.Default(context)

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
	session := sessions.Default(context)

	timeout := session.Get(authenticate.SESSION_TIMEOUT_KEY)
	if timeout != nil {
		timeout = timeout.(int)
	}
	if timeout == nil || timeout == 0 {
		timeout = authenticate.DefaultAuthTimeout
	}
	context.HTML(http.StatusOK, "timeout.tmpl", gin.H{"timeout": timeout})
}

func (authController *AuthController) isUac16() bool {
	return authController.UacKind == "uac16"
}
