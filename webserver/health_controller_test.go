package webserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ONSdigital/blaise-cawi-portal/webserver"
	"github.com/gin-gonic/gin"
)

func TestHealthController(t *testing.T) {
	router := gin.Default()
	healthController := &webserver.HealthController{}
	healthController.AddRoutes(router)

	t.Run("returns healthy response on /health", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "/health", nil)
		if err != nil {
			t.Fatalf("http.NewRequest() error: %v", err)
		}
		router.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}

		var response webserver.Health
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("json.Unmarshal() error: %v", err)
		}
		if !response.Healthy || response.Version != "" {
			t.Fatalf("response = %+v, want healthy=true version=empty", response)
		}
	})

	t.Run("returns versioned healthy response", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "/cawi-portal/v2/health", nil)
		if err != nil {
			t.Fatalf("http.NewRequest() error: %v", err)
		}
		router.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}

		var response webserver.Health
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("json.Unmarshal() error: %v", err)
		}
		if !response.Healthy || response.Version != "v2" {
			t.Fatalf("response = %+v, want healthy=true version=v2", response)
		}
	})

	t.Run("echoes app-engine commands from /_ah routes", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "/_ah/start", nil)
		if err != nil {
			t.Fatalf("http.NewRequest() error: %v", err)
		}
		router.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		if recorder.Body.String() != "\"/start\"" {
			t.Fatalf("body = %q, want %q", recorder.Body.String(), "\"/start\"")
		}
	})
}
