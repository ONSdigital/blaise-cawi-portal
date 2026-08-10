package languagemanager_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ONSdigital/blaise-cawi-portal/languagemanager"
	"github.com/gin-gonic/gin"
)

func TestGetLangFromQuery(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{name: "normalizes uppercase", path: "/?lang=CY", expected: "cy"},
		{name: "supports mixed case", path: "/?lang=En", expected: "en"},
		{name: "returns empty string if query missing", path: "/", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, tt.path, nil)

			if got := languagemanager.GetLangFromQuery(context); got != tt.expected {
				t.Fatalf("GetLangFromQuery() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGetLangFromParam(t *testing.T) {
	tests := []struct {
		name     string
		lang     string
		expected string
	}{
		{name: "normalizes uppercase path param", lang: "CY", expected: "cy"},
		{name: "keeps lowercase path param", lang: "en", expected: "en"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Params = gin.Params{{Key: "lang", Value: tt.lang}}

			if got := languagemanager.GetLangFromParam(context); got != tt.expected {
				t.Fatalf("GetLangFromParam() = %q, want %q", got, tt.expected)
			}
		})
	}

	t.Run("returns empty string if path param is missing", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		if got := languagemanager.GetLangFromParam(context); got != "" {
			t.Fatalf("GetLangFromParam() = %q, want empty string", got)
		}
	})
}
