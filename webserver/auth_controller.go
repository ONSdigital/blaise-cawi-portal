package webserver

import (
	"fmt"
	"net/http"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/utils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	csrf "github.com/utrack/gin-csrf"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
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
			otelgin.HTML(context, http.StatusForbidden, "login.tmpl", gin.H{
				"uac16": authController.isUac16(),
				"error": "Something went wrong, please try again"})
			context.Abort()
		},
	}))
	{
		authGroup.GET("/login", authController.LoginEndpoint)
		authGroup.POST("/login", authController.PostLoginEndpoint)
		authGroup.GET("/logout", authController.LogoutEndpoint)
	}
}

func (authController *AuthController) LoginEndpoint(context *gin.Context) {
	context.Set("csrfSecret", authController.CSRFSecret)
	hasSession, claim := authController.Auth.HasSession(context)
	if hasSession {
		context.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/%s/", claim.UacInfo.InstrumentName))
		return
	}

	otelgin.HTML(context, http.StatusOK, "login.tmpl", gin.H{
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

func (authController *AuthController) isUac16() bool {
	return authController.UacKind == "uac16"
}
