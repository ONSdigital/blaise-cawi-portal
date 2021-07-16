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
}

func (healthController *HealthController) HealthEndpoint(context *gin.Context) {
	version := context.Param("version")
	context.JSON(http.StatusOK, Health{Healthy: true, Version: version})
}
