package webserver

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func InternalServerError(context *gin.Context) {
	context.HTML(http.StatusInternalServerError, "server_error.tmpl", gin.H{})
	context.Abort()
}
