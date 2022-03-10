package webserver

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func InternalServerError(context *gin.Context, welsh bool) {
	context.HTML(http.StatusInternalServerError, "server_error.tmpl", gin.H{"welsh": welsh})
	context.Abort()
}
