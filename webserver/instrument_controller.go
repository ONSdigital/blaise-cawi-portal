package webserver

import (
	"net/http"

	"github.com/ONSdigital/blaise-cawi-portal/login"
	"github.com/gin-gonic/gin"
)

type InstrumentController struct {
	Auth *login.Auth
}

func (instrumentController *InstrumentController) AddRoutes(httpRouter *gin.Engine) {
	instrumentRouter := httpRouter.Group("/:instrumentName")
	{
		instrumentRouter.GET("/", func(context *gin.Context) {
			instrumentController.Auth.Authenticated(context)
			context.JSON(http.StatusOK, gin.H{"authenticated": true})
		})
	}
}
