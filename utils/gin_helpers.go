package utils

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"html"
	"strings"
	"unicode"
)

func SanitizeLogInput(input string) string {
	escapedInput := html.EscapeString(input)
	escapedInput = strings.ReplaceAll(escapedInput, "\n", "")
	escapedInput = strings.ReplaceAll(escapedInput, "\r", "")
	escapedInput = strings.ReplaceAll(escapedInput, "\t", "")
	return strings.Map(func(r rune) rune {
		if !unicode.IsPrint(r) {
			return -1
		}
		return r
	}, escapedInput)
}

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
