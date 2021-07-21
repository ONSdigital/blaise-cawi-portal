package webserver

import (
	"fmt"
	"net/http"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type AuthController struct {
	Auth authenticate.AuthInterface
}

func (authController *AuthController) AddRoutes(httpRouter *gin.Engine) {
	authGroup := httpRouter.Group("/auth")
	{
		authGroup.GET("/login", authController.LoginEndpoint)
		authGroup.POST("/login", authController.PostLoginEndpoint)
		authGroup.GET("/login/postcode", authController.Auth.AuthenticatedStage1, authController.PostcodeEndpoint)
		authGroup.POST("/login/postcode", authController.PostPostcodeEndpoint)
		authGroup.GET("/logout", authController.LogoutEndpoint)
	}
}

func (authController *AuthController) LoginEndpoint(context *gin.Context) {
	hasSession, claim := authController.Auth.HasSession(context)
	if hasSession {
		context.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/%s/", claim.UacInfo.InstrumentName))
		return
	}
	context.HTML(http.StatusOK, "login.tmpl", gin.H{})
}

func (authController *AuthController) PostLoginEndpoint(context *gin.Context) {
	session := sessions.Default(context)

	authController.Auth.Login(context, session)
}

func (authController *AuthController) PostcodeEndpoint(context *gin.Context) {
	context.HTML(http.StatusOK, "postcode.tmpl", gin.H{})
}

func (authController *AuthController) PostPostcodeEndpoint(context *gin.Context) {
	session := sessions.Default(context)

	authController.Auth.LoginPostcode(context, session)
}

func (authController *AuthController) LogoutEndpoint(context *gin.Context) {
	session := sessions.Default(context)

	authController.Auth.Logout(context, session)
}
