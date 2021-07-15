package webserver

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthController struct{}

func (authController *AuthController) AddRoutes(httpRouter *gin.Engine) {
	authGroup := httpRouter.Group("/auth")
	{
		authGroup.GET("/login", authController.LoginEndpoint)
	}
}

func (authController *AuthController) LoginEndpoint(context *gin.Context) {
	context.HTML(http.StatusOK, "login.tmpl", gin.H{})
}
