package utils_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ONSdigital/blaise-cawi-portal/utils"
	"github.com/gin-gonic/gin"
)

func TestGetRequestSource(t *testing.T) {
	t.Run("returns only SourceIP when remote address and client IP match", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost:8000", nil)
		if err != nil {
			t.Fatalf("http.NewRequest() error: %v", err)
		}
		req.RemoteAddr = "1.1.1.1"

		context, engine := gin.CreateTestContext(httptest.NewRecorder())
		engine.AppEngine = true
		context.Request = req

		requestSource := utils.GetRequestSource(context)
		if len(requestSource) != 1 {
			t.Fatalf("len(requestSource) = %d, want 1", len(requestSource))
		}
		if requestSource[0].String != "1.1.1.1" || requestSource[0].Key != "SourceIP" {
			t.Fatalf("requestSource[0] = %+v, want SourceIP=1.1.1.1", requestSource[0])
		}
	})

	t.Run("returns SourceIP and SourceXFF when addresses differ", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost:8000", nil)
		if err != nil {
			t.Fatalf("http.NewRequest() error: %v", err)
		}
		req.RemoteAddr = "1.1.1.1"
		req.Header.Add("X-Appengine-Remote-Addr", "2.2.2.2")

		context, engine := gin.CreateTestContext(httptest.NewRecorder())
		engine.AppEngine = true
		context.Request = req

		requestSource := utils.GetRequestSource(context)
		if len(requestSource) != 2 {
			t.Fatalf("len(requestSource) = %d, want 2", len(requestSource))
		}
		if requestSource[0].String != "1.1.1.1" || requestSource[0].Key != "SourceIP" {
			t.Fatalf("requestSource[0] = %+v, want SourceIP=1.1.1.1", requestSource[0])
		}
		if requestSource[1].String != "2.2.2.2" || requestSource[1].Key != "SourceXFF" {
			t.Fatalf("requestSource[1] = %+v, want SourceXFF=2.2.2.2", requestSource[1])
		}
	})
}

func TestSanitizeLogInput(t *testing.T) {
	t.Run("removes line breaks tabs and non printable characters", func(t *testing.T) {
		input := "row1\nrow2\r\trow3\x00\x1f"
		sanitized := utils.SanitizeLogInput(input)
		if sanitized != "row1row2row3" {
			t.Fatalf("SanitizeLogInput() = %q, want %q", sanitized, "row1row2row3")
		}
	})

	t.Run("escapes html characters", func(t *testing.T) {
		input := `<script>alert("x")</script>`
		sanitized := utils.SanitizeLogInput(input)
		expected := "&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;"
		if sanitized != expected {
			t.Fatalf("SanitizeLogInput() = %q, want %q", sanitized, expected)
		}
	})

	t.Run("keeps printable unicode", func(t *testing.T) {
		input := "Arolygon Cymru"
		sanitized := utils.SanitizeLogInput(input)
		if sanitized != input {
			t.Fatalf("SanitizeLogInput() = %q, want %q", sanitized, input)
		}
	})
}
