package csrf

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ONSdigital/blaise-cawi-portal/sessionkeys"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// failSession is a sessions.Session whose Save always returns an error.
type failSession struct{}

func (failSession) ID() string                                      { return "" }
func (failSession) Get(key interface{}) interface{}                 { return nil }
func (failSession) Set(key interface{}, val interface{})            {}
func (failSession) Delete(key interface{})                          {}
func (failSession) Clear()                                          {}
func (failSession) AddFlash(value interface{}, vars ...string)      {}
func (failSession) Flashes(vars ...string) []interface{}            { return nil }
func (failSession) Options(sessions.Options)                        {}
func (failSession) Save() error                                     { return errors.New("store unavailable") }

func buildRouter(csrfManager *DefaultCSRFManager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := cookie.NewStore([]byte("store-secret"))

	if csrfManager.SessionName == "" {
		router.Use(sessions.Sessions(sessionkeys.SessionName, store))
	} else {
		router.Use(sessions.SessionsMany([]string{csrfManager.SessionName, "other"}, store))
	}

	router.GET("/token", func(c *gin.Context) {
		c.String(http.StatusOK, csrfManager.GetToken(c))
	})

	handler := func(c *gin.Context) {
		c.Header("X-Handled", "true")
		if secret, ok := c.Get(csrfSecret); ok {
			c.Header("X-CSRF-Secret", secret.(string))
		}
		c.Status(http.StatusNoContent)
	}

	router.GET("/protected", csrfManager.Middleware(), handler)
	router.POST("/protected", csrfManager.Middleware(), handler)

	return router
}

func performRequest(router *gin.Engine, request *http.Request, sessionCookie *http.Cookie) *httptest.ResponseRecorder {
	if sessionCookie != nil {
		request.AddCookie(sessionCookie)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	return recorder
}

func TestMiddlewareAllowsIgnoredMethodsByDefault(t *testing.T) {
	csrfManager := &DefaultCSRFManager{Secret: "secret"}
	router := buildRouter(csrfManager)

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	response := performRequest(router, request, nil)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, response.Code)
	}
	if response.Header().Get("X-Handled") != "true" {
		t.Fatal("expected request to reach protected handler")
	}
	if response.Header().Get("X-CSRF-Secret") != "secret" {
		t.Fatal("expected middleware to set csrf secret in context")
	}
}

func TestMiddlewareDefaultErrorFuncRejectsPostWithoutSalt(t *testing.T) {
	csrfManager := &DefaultCSRFManager{Secret: "secret"}
	router := buildRouter(csrfManager)

	request := httptest.NewRequest(http.MethodPost, "/protected", nil)
	response := performRequest(router, request, nil)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
	if response.Header().Get("X-Handled") != "" {
		t.Fatal("expected middleware to stop request before protected handler")
	}
}

func TestMiddlewareRejectsPostWithoutSalt(t *testing.T) {
	csrfManager := &DefaultCSRFManager{
		Secret: "secret",
		ErrorFunc: func(c *gin.Context) {
			c.Status(http.StatusForbidden)
			c.Abort()
		},
	}
	router := buildRouter(csrfManager)

	request := httptest.NewRequest(http.MethodPost, "/protected", nil)
	response := performRequest(router, request, nil)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
	if response.Header().Get("X-Handled") != "" {
		t.Fatal("expected middleware to stop request before protected handler")
	}
}

func TestMiddlewareRejectsMismatchedToken(t *testing.T) {
	csrfManager := &DefaultCSRFManager{
		Secret: "secret",
		ErrorFunc: func(c *gin.Context) {
			c.Status(http.StatusForbidden)
			c.Abort()
		},
	}
	router := buildRouter(csrfManager)

	tokenRequest := httptest.NewRequest(http.MethodGet, "/token", nil)
	tokenResponse := performRequest(router, tokenRequest, nil)

	if tokenResponse.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, tokenResponse.Code)
	}

	sessionCookie := tokenResponse.Result().Cookies()[0]
	request := httptest.NewRequest(http.MethodPost, "/protected?_csrf=wrong-token", nil)
	response := performRequest(router, request, sessionCookie)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
	if response.Header().Get("X-Handled") != "" {
		t.Fatal("expected middleware to stop request before protected handler")
	}
}

func TestMiddlewareAcceptsValidTokenWithNamedSession(t *testing.T) {
	csrfManager := &DefaultCSRFManager{
		SessionName: sessionkeys.SessionName,
		Secret:      "secret",
		ErrorFunc: func(c *gin.Context) {
			c.Status(http.StatusForbidden)
			c.Abort()
		},
	}
	router := buildRouter(csrfManager)

	tokenRequest := httptest.NewRequest(http.MethodGet, "/token", nil)
	tokenResponse := performRequest(router, tokenRequest, nil)

	if tokenResponse.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, tokenResponse.Code)
	}

	tokenBody, err := io.ReadAll(tokenResponse.Body)
	if err != nil {
		t.Fatalf("failed to read token body: %v", err)
	}
	token := string(tokenBody)
	sessionCookie := tokenResponse.Result().Cookies()[0]

	request := httptest.NewRequest(http.MethodPost, "/protected", nil)
	request.Header.Set("X-CSRF-TOKEN", token)
	response := performRequest(router, request, sessionCookie)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, response.Code)
	}
	if response.Header().Get("X-Handled") != "true" {
		t.Fatal("expected request to reach protected handler")
	}
}

func TestDefaultTokenGetterOrder(t *testing.T) {
	tests := []struct {
		name       string
		requestURL string
		body       string
		headers    map[string]string
		expected   string
	}{
		{
			name:       "prefers form value",
			requestURL: "/?" + "_csrf=query-token",
			body:       "_csrf=form-token",
			headers: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
				"X-CSRF-TOKEN": "header-token",
			},
			expected: "form-token",
		},
		{
			name:       "uses query value when form missing",
			requestURL: "/?" + "_csrf=query-token",
			expected:   "query-token",
		},
		{
			name:       "uses x csrf header when form and query missing",
			requestURL: "/",
			headers: map[string]string{
				"X-CSRF-TOKEN": "header-token",
			},
			expected: "header-token",
		},
		{
			name:       "uses xsrf header as final fallback",
			requestURL: "/",
			headers: map[string]string{
				"X-XSRF-TOKEN": "xsrf-token",
			},
			expected: "xsrf-token",
		},
		{
			name:       "returns empty string when no token is present",
			requestURL: "/",
			expected:   "",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)

			var body io.Reader
			if testCase.body != "" {
				body = strings.NewReader(testCase.body)
			}
			request := httptest.NewRequest(http.MethodPost, testCase.requestURL, body)
			for key, value := range testCase.headers {
				request.Header.Set(key, value)
			}
			context.Request = request

			token := defaultTokenGetter(context)
			if token != testCase.expected {
				t.Fatalf("expected token %q, got %q", testCase.expected, token)
			}
		})
	}
}

func TestGetTokenAbortsWithInternalServerErrorOnSessionSaveFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	csrfManager := &DefaultCSRFManager{Secret: "secret"}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/token", nil)
	c.Set(sessions.DefaultKey, failSession{})

	token := csrfManager.GetToken(c)

	if token != "" {
		t.Fatalf("expected empty token on save failure, got %q", token)
	}
	if !c.IsAborted() {
		t.Fatal("expected context to be aborted on session save failure")
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}
}
