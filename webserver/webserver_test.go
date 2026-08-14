package webserver

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ONSdigital/blaise-cawi-portal/csrf"
	"github.com/ONSdigital/blaise-cawi-portal/languagemanager"
	"github.com/ONSdigital/blaise-cawi-portal/sessionkeys"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/api/idtoken"
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
		host       string
		referer    string
		wantStatus int
		wantLoc    string
	}{
		{name: "welsh language", method: http.MethodPost, path: "/language/welsh", wantStatus: http.StatusOK},
		{name: "english language", method: http.MethodPost, path: "/language/english", wantStatus: http.StatusOK},
		{name: "welsh language fallback via get", method: http.MethodGet, path: "/language/welsh", wantStatus: http.StatusSeeOther, wantLoc: "/"},
		{name: "welsh language redirect via same host referer", method: http.MethodGet, path: "/language/welsh", host: "portal.test", referer: "https://portal.test/instruments?lang=cy", wantStatus: http.StatusSeeOther, wantLoc: "/instruments?lang=cy"},
		{name: "welsh language rejects scheme-relative target", method: http.MethodGet, path: "/language/welsh", host: "portal.test", referer: "https://portal.test//evil.example?lang=cy", wantStatus: http.StatusSeeOther, wantLoc: "/"},
		{name: "welsh language rejects cross-host referer", method: http.MethodGet, path: "/language/welsh", host: "portal.test", referer: "https://evil.example/instruments?lang=cy", wantStatus: http.StatusSeeOther, wantLoc: "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req, err := http.NewRequest(tt.method, tt.path, nil)
			if err != nil {
				t.Fatalf("http.NewRequest() error: %v", err)
			}
			if tt.host != "" {
				req.Host = tt.host
			}
			if tt.referer != "" {
				req.Header.Set("Referer", tt.referer)
			}
			router.ServeHTTP(recorder, req)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}

			if tt.wantLoc != "" {
				if got := recorder.Header().Get("Location"); got != tt.wantLoc {
					t.Fatalf("Location header = %q, want %q", got, tt.wantLoc)
				}
			}
		})
	}

	if len(languageManager.setCalls) != len(tests) {
		t.Fatalf("SetWelsh call count = %d, want %d", len(languageManager.setCalls), len(tests))
	}
	if languageManager.setCalls[0] != true || languageManager.setCalls[1] != false || languageManager.setCalls[2] != true || languageManager.setCalls[3] != true || languageManager.setCalls[4] != true || languageManager.setCalls[5] != true {
		t.Fatalf("SetWelsh calls = %v, want [true false true true true true]", languageManager.setCalls)
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

func TestNewHTTPClientUsesConfiguredTimeout(t *testing.T) {
	client := newHTTPClient()

	if client.Timeout != httpClientTimeout {
		t.Fatalf("client.Timeout = %v, want %v", client.Timeout, httpClientTimeout)
	}
}

func TestShouldUseSecureCookies(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		want   bool
	}{
		{
			name:   "uses secure cookies outside dev mode",
			config: &Config{DevMode: false, EnableHTTPS: false},
			want:   true,
		},
		{
			name:   "disables secure cookies in dev mode over http",
			config: &Config{DevMode: true, EnableHTTPS: false},
			want:   false,
		},
		{
			name:   "keeps secure cookies in dev mode when https enabled",
			config: &Config{DevMode: true, EnableHTTPS: true},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldUseSecureCookies(tt.config)
			if got != tt.want {
				t.Fatalf("shouldUseSecureCookies() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetupRouterBuildsRouterWithCoreRoutes(t *testing.T) {
	withRepoRootAsWorkingDirectory(t)

	originalIDTokenNewClient := idTokenNewClient
	idTokenNewClient = func(_ context.Context, _ string, _ ...idtoken.ClientOption) (*http.Client, error) {
		return &http.Client{}, nil
	}
	t.Cleanup(func() {
		idTokenNewClient = originalIDTokenNewClient
	})

	server := &Server{Config: &Config{
		DevMode:          true,
		SessionSecret:    "session-secret",
		EncryptionSecret: "0123456789abcdef",
		CatiURL:          "https://cati.test",
		JWTSecret:        "jwt-secret",
		BusURL:           "https://bus.test",
		BusClientID:      "test-audience",
		BlaiseRestAPI:    "https://rest.test",
		Serverpark:       "gusty",
		UACKind:          "uac",
	}}

	router, err := server.SetupRouter()
	if err != nil {
		t.Fatalf("SetupRouter() error: %v", err)
	}

	if router == nil {
		t.Fatal("SetupRouter() returned nil router")
	}

	if router.TrustedPlatform != gin.PlatformGoogleAppEngine {
		t.Fatalf("TrustedPlatform = %q, want %q", router.TrustedPlatform, gin.PlatformGoogleAppEngine)
	}

	routes := router.Routes()
	for _, expectedRoute := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/"},
		{method: http.MethodGet, path: "/auth/login"},
		{method: http.MethodGet, path: "/health"},
		{method: http.MethodPost, path: "/language/:lang"},
		{method: http.MethodGet, path: "/:instrumentName/"},
	} {
		if !routeExists(routes, expectedRoute.method, expectedRoute.path) {
			t.Fatalf("expected route %s %s to be registered", expectedRoute.method, expectedRoute.path)
		}
	}
}

func TestSetupRouterReturnsErrorWhenBusClientCreationFails(t *testing.T) {
	withRepoRootAsWorkingDirectory(t)

	originalIDTokenNewClient := idTokenNewClient
	idTokenNewClient = func(_ context.Context, _ string, _ ...idtoken.ClientOption) (*http.Client, error) {
		return nil, errors.New("unable to create token client")
	}
	t.Cleanup(func() {
		idTokenNewClient = originalIDTokenNewClient
	})

	server := &Server{Config: &Config{
		DevMode:          true,
		SessionSecret:    "session-secret",
		EncryptionSecret: "0123456789abcdef",
		CatiURL:          "https://cati.test",
		JWTSecret:        "jwt-secret",
		BusURL:           "https://bus.test",
		BusClientID:      "test-audience",
		BlaiseRestAPI:    "https://rest.test",
		Serverpark:       "gusty",
		UACKind:          "uac",
	}}

	router, err := server.SetupRouter()
	if err == nil {
		t.Fatal("SetupRouter() expected error, got nil")
	}
	if router != nil {
		t.Fatalf("router = %v, want nil", router)
	}
	if !strings.Contains(err.Error(), "error creating bus client") {
		t.Fatalf("error = %q, want to contain %q", err.Error(), "error creating bus client")
	}
}

func routeExists(routes gin.RoutesInfo, method string, path string) bool {
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return true
		}
	}

	return false
}

func withRepoRootAsWorkingDirectory(t *testing.T) {
	t.Helper()

	originalWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error: %v", err)
	}

	if _, err = os.Stat(filepath.Join(originalWorkingDirectory, "templates")); err == nil {
		return
	}

	repoRoot := filepath.Clean(filepath.Join(originalWorkingDirectory, ".."))
	if _, err = os.Stat(filepath.Join(repoRoot, "templates")); err != nil {
		t.Fatalf("could not locate templates directory from %q: %v", originalWorkingDirectory, err)
	}

	if err = os.Chdir(repoRoot); err != nil {
		t.Fatalf("os.Chdir(%q) error: %v", repoRoot, err)
	}

	t.Cleanup(func() {
		if chdirErr := os.Chdir(originalWorkingDirectory); chdirErr != nil {
			t.Errorf("cleanup os.Chdir(%q) error: %v", originalWorkingDirectory, chdirErr)
		}
	})
}
