package webserver

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type SecurityController struct{}

func (securityController *SecurityController) AddRoutes(httpRouter *gin.Engine) {
	httpRouter.Use(func(context *gin.Context) {
		if context.Request.Method == http.MethodTrace {
			context.AbortWithStatus(http.StatusMethodNotAllowed)
		}
	})
}
