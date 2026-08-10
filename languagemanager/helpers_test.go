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
