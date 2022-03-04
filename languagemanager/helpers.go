package languagemanager

import "github.com/gin-gonic/gin"

func IsWelshFromParam(context *gin.Context) bool {
	lang, langPresent := context.GetQuery("lang")
	if langPresent && lang == "cy" {
		return true
	}
	return false
}

func GetLangFromParam(context *gin.Context) string {
	lang, langPresent := context.GetQuery("lang")
	if langPresent {
		return lang
	}
	return ""
}
