package languagemanager

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func IsWelshFromParam(context *gin.Context) bool {
	lang, langPresent := context.GetQuery("lang")
	if langPresent && lang == "cy" {
		return true
	}
	return false
}

func GetLangFromQuery(context *gin.Context) string {
	lang, langPresent := context.GetQuery("lang")
	if langPresent {
		return strings.ToLower(lang)
	}
	return ""
}

func GetLangFromParam(context *gin.Context) string {
	lang, langPresent := context.Params.Get("lang")
	if langPresent {
		return strings.ToLower(lang)
	}
	return ""
}
