package webserver

import (
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ONSdigital/blaise-cawi-portal/csrf"
	"github.com/ONSdigital/blaise-cawi-portal/languagemanager"
	"github.com/ONSdigital/blaise-cawi-portal/sessionkeys"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type testLanguageManager struct {
	isWelsh     bool
	setCalls    []bool
	isWelshHits int
}

func (manager *testLanguageManager) IsWelsh(*gin.Context) bool {
	manager.isWelshHits++
	return manager.isWelsh
}

func (manager *testLanguageManager) SetWelsh(_ *gin.Context, welsh bool) {
	manager.setCalls = append(manager.setCalls, welsh)
	manager.isWelsh = welsh
}

func (manager *testLanguageManager) LanguageError(errMap map[string]string, _ *gin.Context) string {
	if manager.isWelsh {
		return errMap["welsh"]
	}
	return errMap["english"]
}

func TestWebserverHelpers(t *testing.T) {
	t.Run("loads config from required env vars and defaults", func(t *testing.T) {
		env := map[string]string{
			"SESSION_SECRET":    "session-secret",
			"ENCRYPTION_SECRET": "0123456789abcdef",
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

		config, err := LoadConfig()
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

		config, err := LoadConfig()
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
			"ENCRYPTION_SECRET": "0123456789abcdef",
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

		config, err := LoadConfig()
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
		logger, err := NewLogger(&Config{DevMode: true})
		if err != nil || logger == nil {
			t.Fatalf("NewLogger() = (%v, %v), want non-nil logger and nil error", logger, err)
		}
	})

	t.Run("creates logger in non-dev mode", func(t *testing.T) {
		logger, err := NewLogger(&Config{DevMode: false, Debug: true})
		if err != nil || logger == nil {
			t.Fatalf("NewLogger() = (%v, %v), want non-nil logger and nil error", logger, err)
		}
	})

	t.Run("creates csrf manager using session config", func(t *testing.T) {
		config := &Config{SessionSecret: "session-secret"}
		logger := zap.NewNop()
		languageManager := &languagemanager.Manager{SessionName: sessionkeys.LanguageSessionName}

		manager := NewCSRFManager(config, logger, languageManager)
		typedManager, ok := manager.(*csrf.DefaultCSRFManager)
		if !ok {
			t.Fatal("manager is not *csrf.DefaultCSRFManager")
		}
		if typedManager.SessionName != sessionkeys.SessionName || typedManager.Secret != "session-secret" || typedManager.ErrorFunc == nil {
			t.Fatalf("typedManager = %+v, expected session config and error func", typedManager)
		}
	})

	t.Run("uses in-memory cookie sessions in dev mode", func(t *testing.T) {
		config := &Config{DevMode: true, SessionSecret: "session-secret", EncryptionSecret: "0123456789abcdef"}
		store, err := UserSessionStore(config)
		if err != nil {
			t.Fatalf("UserSessionStore() error: %v", err)
		}
		if !strings.Contains(fmt.Sprintf("%T", store), "cookie") {
			t.Fatalf("store type = %T, expected cookie store", store)
		}
	})

	t.Run("wraps welsh state for templates", func(t *testing.T) {
		payload := WrapWelsh(true)
		if value, ok := payload["welsh"]; !ok || value != true {
			t.Fatalf("payload = %+v, want welsh=true", payload)
		}
	})
}

func TestConfigureSessionMiddlewareRegistersStores(t *testing.T) {
	server := &Server{Config: &Config{DevMode: true, SessionSecret: "session-secret", EncryptionSecret: "0123456789abcdef"}}
	router := gin.New()

	if err := server.configureSessionMiddleware(router); err != nil {
		t.Fatalf("configureSessionMiddleware() error: %v", err)
	}

	sessionNames := []string{
		sessionkeys.SessionName,
		sessionkeys.UserSessionName,
		sessionkeys.SessionValidationName,
		sessionkeys.LanguageSessionName,
	}

	router.GET("/session-check", func(context *gin.Context) {
		for index, sessionName := range sessionNames {
			session := sessions.DefaultMany(context, sessionName)
			session.Set("probe", index)
			if err := session.Save(); err != nil {
				context.String(http.StatusInternalServerError, err.Error())
				return
			}
		}
		context.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/session-check", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error: %v", err)
	}
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body=%q", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}

	cookieNames := map[string]bool{}
	for _, cookie := range recorder.Result().Cookies() {
		cookieNames[cookie.Name] = true
	}

	for _, sessionName := range sessionNames {
		if !cookieNames[sessionName] {
			t.Fatalf("missing cookie for session store %s", sessionName)
		}
	}
}

func TestRegisterUtilityRoutesLanguageAndNoRoute(t *testing.T) {
	languageManager := &testLanguageManager{}
	router := gin.New()
	router.SetFuncMap(template.FuncMap{"WrapWelsh": WrapWelsh})
	router.LoadHTMLGlob("../templates/*")

	registerUtilityRoutes(router, &AuthController{}, languageManager)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{name: "welsh language", method: http.MethodPost, path: "/language/welsh", wantStatus: http.StatusOK},
		{name: "english language", method: http.MethodPost, path: "/language/english", wantStatus: http.StatusOK},
		{name: "get not allowed", method: http.MethodGet, path: "/language/welsh", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req, err := http.NewRequest(tt.method, tt.path, nil)
			if err != nil {
				t.Fatalf("http.NewRequest() error: %v", err)
			}
			router.ServeHTTP(recorder, req)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
		})
	}

	if len(languageManager.setCalls) != 2 {
		t.Fatalf("SetWelsh call count = %d, want 2", len(languageManager.setCalls))
	}
	if languageManager.setCalls[0] != true || languageManager.setCalls[1] != false {
		t.Fatalf("SetWelsh calls = %v, want [true false]", languageManager.setCalls)
	}

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/not-a-route", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error: %v", err)
	}
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if languageManager.isWelshHits == 0 {
		t.Fatal("expected NoRoute handler to query language manager")
	}
}
