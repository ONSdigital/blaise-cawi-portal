package webserver_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ONSdigital/blaise-cawi-portal/csrf"
	"github.com/ONSdigital/blaise-cawi-portal/languagemanager"
	"github.com/ONSdigital/blaise-cawi-portal/sessionkeys"
	"github.com/ONSdigital/blaise-cawi-portal/webserver"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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

func TestWebserverHelpers(t *testing.T) {
	t.Run("loads config from required env vars and defaults", func(t *testing.T) {
		env := map[string]string{
			"SESSION_SECRET":    "session-secret",
			"ENCRYPTION_SECRET": "encryption-secret",
			"CATI_URL":          "https://cati.test",
			"JWT_SECRET":        "jwt-secret",
			"BUS_URL":           "https://bus.test",
			"BUS_CLIENT_ID":     "bus-client-id",
			"BLAISE_REST_API":   "https://rest.test",
		}

		for key, value := range env {
			original, existed := os.LookupEnv(key)
			if err := os.Setenv(key, value); err != nil {
				t.Fatalf("os.Setenv(%s) error: %v", key, err)
			}
			if existed {
				t.Cleanup(func(k, v string) func() {
					return func() {
						if err := os.Setenv(k, v); err != nil {
							t.Errorf("cleanup os.Setenv(%s) error: %v", k, err)
						}
					}
				}(key, original))
			} else {
				t.Cleanup(func(k string) func() {
					return func() {
						if err := os.Unsetenv(k); err != nil {
							t.Errorf("cleanup os.Unsetenv(%s) error: %v", k, err)
						}
					}
				}(key))
			}
		}

		config, err := webserver.LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error: %v", err)
		}
		if config.Port != "8082" || config.Serverpark != "gusty" || config.UACKind != "uac" || config.SessionSecret != "session-secret" {
			t.Fatalf("unexpected config: %+v", config)
		}
	})

	t.Run("returns error when required config is missing", func(t *testing.T) {
		requiredKeys := []string{"SESSION_SECRET", "ENCRYPTION_SECRET", "CATI_URL", "JWT_SECRET", "BUS_URL", "BUS_CLIENT_ID", "BLAISE_REST_API"}

		for _, key := range requiredKeys {
			original, existed := os.LookupEnv(key)
			if err := os.Unsetenv(key); err != nil {
				t.Fatalf("os.Unsetenv(%s) error: %v", key, err)
			}
			if existed {
				t.Cleanup(func(k, v string) func() {
					return func() {
						if err := os.Setenv(k, v); err != nil {
							t.Errorf("cleanup os.Setenv(%s) error: %v", k, err)
						}
					}
				}(key, original))
			}
		}

		config, err := webserver.LoadConfig()
		if config != nil {
			t.Fatalf("config = %+v, want nil", config)
		}
		if err == nil {
			t.Fatal("LoadConfig() expected error, got nil")
		}
	})

	t.Run("returns error when required config is empty", func(t *testing.T) {
		env := map[string]string{
			"SESSION_SECRET":    "session-secret",
			"ENCRYPTION_SECRET": "encryption-secret",
			"CATI_URL":          "https://cati.test",
			"JWT_SECRET":        " ",
			"BUS_URL":           "",
			"BUS_CLIENT_ID":     "bus-client-id",
			"BLAISE_REST_API":   "https://rest.test",
		}

		for key, value := range env {
			original, existed := os.LookupEnv(key)
			if err := os.Setenv(key, value); err != nil {
				t.Fatalf("os.Setenv(%s) error: %v", key, err)
			}
			if existed {
				t.Cleanup(func(k, v string) func() {
					return func() {
						if err := os.Setenv(k, v); err != nil {
							t.Errorf("cleanup os.Setenv(%s) error: %v", k, err)
						}
					}
				}(key, original))
			} else {
				t.Cleanup(func(k string) func() {
					return func() {
						if err := os.Unsetenv(k); err != nil {
							t.Errorf("cleanup os.Unsetenv(%s) error: %v", k, err)
						}
					}
				}(key))
			}
		}

		config, err := webserver.LoadConfig()
		if config != nil {
			t.Fatalf("config = %+v, want nil", config)
		}
		if err == nil {
			t.Fatal("LoadConfig() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "JWT_SECRET") || !strings.Contains(err.Error(), "BUS_URL") {
			t.Fatalf("error %q missing expected fields", err.Error())
		}
	})

	t.Run("creates logger in dev mode", func(t *testing.T) {
		logger, err := webserver.NewLogger(&webserver.Config{DevMode: true})
		if err != nil || logger == nil {
			t.Fatalf("NewLogger() = (%v, %v), want non-nil logger and nil error", logger, err)
		}
	})

	t.Run("creates logger in non-dev mode", func(t *testing.T) {
		logger, err := webserver.NewLogger(&webserver.Config{DevMode: false, Debug: true})
		if err != nil || logger == nil {
			t.Fatalf("NewLogger() = (%v, %v), want non-nil logger and nil error", logger, err)
		}
	})

	t.Run("creates csrf manager using session config", func(t *testing.T) {
		config := &webserver.Config{SessionSecret: "session-secret"}
		logger := zap.NewNop()
		languageManager := &languagemanager.Manager{SessionName: sessionkeys.LanguageSessionName}

		manager := webserver.NewCSRFManager(config, logger, languageManager)
		typedManager, ok := manager.(*csrf.DefaultCSRFManager)
		if !ok {
			t.Fatal("manager is not *csrf.DefaultCSRFManager")
		}
		if typedManager.SessionName != sessionkeys.SessionName || typedManager.Secret != "session-secret" || typedManager.ErrorFunc == nil {
			t.Fatalf("typedManager = %+v, expected session config and error func", typedManager)
		}
	})

	t.Run("uses in-memory cookie sessions in dev mode", func(t *testing.T) {
		config := &webserver.Config{DevMode: true, SessionSecret: "session-secret", EncryptionSecret: "encryption-secret"}
		store, err := webserver.UserSessionStore(config)
		if err != nil {
			t.Fatalf("UserSessionStore() error: %v", err)
		}
		if !strings.Contains(fmt.Sprintf("%T", store), "cookie") {
			t.Fatalf("store type = %T, expected cookie store", store)
		}
	})

	t.Run("wraps welsh state for templates", func(t *testing.T) {
		payload := webserver.WrapWelsh(true)
		if value, ok := payload["welsh"]; !ok || value != true {
			t.Fatalf("payload = %+v, want welsh=true", payload)
		}
	})
}
