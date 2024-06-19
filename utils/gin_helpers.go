package utils

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"strings"
)

func GetRequestSource(context *gin.Context) []zap.Field {
	var requestSource []zap.Field
	remoteAddress := context.Request.RemoteAddr
	clientIP := context.ClientIP()

	requestSource = append(requestSource, zap.String("SourceIP", remoteAddress))

	if remoteAddress != clientIP && clientIP != "" {

		clientIP = strings.ReplaceAll(clientIP, "\n", "")
		clientIP = strings.ReplaceAll(clientIP, "\r", "")

		requestSource = append(requestSource, zap.String("SourceXFF", clientIP))
	}

	return requestSource
}
