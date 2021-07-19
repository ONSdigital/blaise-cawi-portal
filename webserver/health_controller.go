package webserver

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Health struct {
	Healthy bool   `json:"healthy"`
	Version string `json:"version,omitempty"`
}

type HealthController struct {
}

func (healthController *HealthController) AddRoutes(httpRouter *gin.Engine) {
	httpRouter.GET("/health", healthController.HealthEndpoint)
	httpRouter.GET("/cawi-portal/:version/health", healthController.HealthEndpoint)
	httpRouter.GET("/_ah/*command", func(context *gin.Context) {
		command := context.Param("command")
		context.JSON(http.StatusOK, command)
	})
}

func (healthController *HealthController) HealthEndpoint(context *gin.Context) {
	version := context.Param("version")
	context.JSON(http.StatusOK, Health{Healthy: true, Version: version})
}
