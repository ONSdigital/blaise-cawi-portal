package webserver

import (
	"fmt"
	"net/http"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	csrf "github.com/utrack/gin-csrf"
)

type AuthController struct {
	Auth       authenticate.AuthInterface
	CSRFSecret string
}

func (authController *AuthController) AddRoutes(httpRouter *gin.Engine) {
	authGroup := httpRouter.Group("/auth")
	authGroup.Use(csrf.Middleware(csrf.Options{
		Secret: authController.CSRFSecret,
		ErrorFunc: func(context *gin.Context) {
			context.String(400, "CSRF token mismatch")
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
	context.HTML(http.StatusOK, "login.tmpl", gin.H{"csrf_token": csrf.GetToken(context)})
}

func (authController *AuthController) PostLoginEndpoint(context *gin.Context) {
	session := sessions.Default(context)

	authController.Auth.Login(context, session)
}

func (authController *AuthController) LogoutEndpoint(context *gin.Context) {
	session := sessions.Default(context)

	authController.Auth.Logout(context, session)
}
