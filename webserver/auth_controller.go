package webserver

import (
	"net/http"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type AuthController struct {
	Auth *authenticate.Auth
}

func (authController *AuthController) AddRoutes(httpRouter *gin.Engine) {
	authGroup := httpRouter.Group("/auth")
	{
		authGroup.GET("/login", authController.LoginEndpoint)
		authGroup.POST("/login", authController.PostLoginEndpoint)
	}
}

func (authController *AuthController) LoginEndpoint(context *gin.Context) {
	context.HTML(http.StatusOK, "login.tmpl", gin.H{})
}

func (authController *AuthController) PostLoginEndpoint(context *gin.Context) {
	session := sessions.Default(context)

	authController.Auth.Login(context, session)
}
