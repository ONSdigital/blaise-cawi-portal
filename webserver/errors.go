package webserver

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func InternalServerError(context *gin.Context) {
	otelgin.HTML(context, http.StatusInternalServerError, "server_error.tmpl", gin.H{})
	context.Abort()
}
