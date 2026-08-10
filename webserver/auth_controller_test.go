package webserver_test

import (
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	authmocks "github.com/ONSdigital/blaise-cawi-portal/authenticate/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/ONSdigital/blaise-cawi-portal/csrf"
	languagemocks "github.com/ONSdigital/blaise-cawi-portal/languagemanager/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/webserver"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type authControllerHarness struct {
	router              *gin.Engine
	mockAuth            *authmocks.AuthInterface
	csrfManager         *csrf.DefaultCSRFManager
	languageManagerMock *languagemocks.LanguageManagerInterface
	authController      *webserver.AuthController
	observedLogs        *observer.ObservedLogs
	config              *webserver.Config
}

func newAuthControllerHarness(t *testing.T) *authControllerHarness {
	t.Helper()

	mockAuth := &authmocks.AuthInterface{}
	csrfManager := &csrf.DefaultCSRFManager{Secret: "fwibble", SessionName: "session"}
	languageManagerMock := &languagemocks.LanguageManagerInterface{}
	config := &webserver.Config{UACKind: "uac16"}

	var observedZapCore zapcore.Core
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore)
	_ = observedLogger.Sync()

	csrfManager.ErrorFunc = webserver.CSRFErrorFunc(csrfManager, config, observedLogger, languageManagerMock)
	router := gin.Default()
	store := cookie.NewStore([]byte("secret"))
	router.Use(sessions.SessionsMany([]string{"session", "user_session", "session_validation", "language_session"}, store))
	router.SetFuncMap(template.FuncMap{"WrapWelsh": webserver.WrapWelsh})
	router.LoadHTMLGlob("../templates/*")

	authController := &webserver.AuthController{
		Auth:            mockAuth,
		CSRFManager:     csrfManager,
		LanguageManager: languageManagerMock,
		Logger:          observedLogger,
	}
	authController.AddRoutes(router)

	return &authControllerHarness{
		router:              router,
		mockAuth:            mockAuth,
		csrfManager:         csrfManager,
		languageManagerMock: languageManagerMock,
		authController:      authController,
		observedLogs:        observedLogs,
		config:              config,
	}
}

func (h *authControllerHarness) get(path string) (*httptest.ResponseRecorder, error) {
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	h.router.ServeHTTP(recorder, req)
	return recorder, nil
}

func (h *authControllerHarness) post(path string, body string, headers map[string]string) (*httptest.ResponseRecorder, error) {
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	h.router.ServeHTTP(recorder, req)
	return recorder, nil
}

func (h *authControllerHarness) csrfTokenAndCookie(t *testing.T) (string, string) {
	t.Helper()
	var csrfToken string
	h.router.GET("/token", func(c *gin.Context) {
		csrfToken = h.csrfManager.GetToken(c)
	})

	recorder, err := h.get("/token")
	if err != nil {
		t.Fatalf("get(/token) error: %v", err)
	}
	return csrfToken, recorder.Header().Get("Set-Cookie")
}

func TestAuthControllerLoginEndpoint(t *testing.T) {
	const (
		instrumentName = "foobar"
		caseID         = "fizzbuzz"
	)

	t.Run("without active session returns login in english", func(t *testing.T) {
		h := newAuthControllerHarness(t)
		h.mockAuth.On("HasSession", mock.Anything).Return(false, nil)
		h.mockAuth.On("IsUAC16").Return(false)
		h.languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
		h.languageManagerMock.On("SetWelsh", mock.Anything, mock.Anything).Return()

		recorder, err := h.get("/auth/login")
		if err != nil {
			t.Fatalf("get(/auth/login) error: %v", err)
		}
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		if !strings.Contains(recorder.Body.String(), `<html lang="en">`) || !strings.Contains(recorder.Body.String(), `Access study`) {
			t.Fatalf("unexpected response body: %s", recorder.Body.String())
		}
	})

	t.Run("without active session returns login in welsh", func(t *testing.T) {
		h := newAuthControllerHarness(t)
		h.mockAuth.On("HasSession", mock.Anything).Return(false, nil)
		h.mockAuth.On("IsUAC16").Return(false)
		h.languageManagerMock.On("IsWelsh", mock.Anything).Return(true)
		h.languageManagerMock.On("SetWelsh", mock.Anything, mock.Anything).Return()

		recorder, err := h.get("/auth/login?lang=cy")
		if err != nil {
			t.Fatalf("get(/auth/login?lang=cy) error: %v", err)
		}
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		if !strings.Contains(recorder.Body.String(), `<html lang="cy">`) || !strings.Contains(recorder.Body.String(), `Agor yr astudiaeth`) {
			t.Fatalf("unexpected response body: %s", recorder.Body.String())
		}
	})

	t.Run("with active session redirects to instrument", func(t *testing.T) {
		h := newAuthControllerHarness(t)
		h.languageManagerMock.On("SetWelsh", mock.Anything, mock.Anything).Return()
		h.mockAuth.On("HasSession", mock.Anything).Return(true, &authenticate.UACClaims{UACInfo: busapi.UACInfo{InstrumentName: instrumentName, CaseID: caseID}}, nil)

		recorder, err := h.get("/auth/login")
		if err != nil {
			t.Fatalf("get(/auth/login) error: %v", err)
		}
		if recorder.Code != http.StatusTemporaryRedirect {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusTemporaryRedirect)
		}
		if location := recorder.Header().Values("Location"); len(location) != 1 || location[0] != fmt.Sprintf("/%s/", instrumentName) {
			t.Fatalf("Location = %v, want /%s/", location, instrumentName)
		}
	})
}

func TestAuthControllerPostLoginEndpoint(t *testing.T) {
	t.Run("without CSRF in english", func(t *testing.T) {
		h := newAuthControllerHarness(t)
		h.mockAuth.On("Login", mock.Anything, mock.Anything).Return()
		h.languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
		h.languageManagerMock.On("LanguageError", authenticate.CSRF_ERR, mock.Anything).Return("Request timed out, please try again")

		recorder, err := h.post("/auth/login", "", nil)
		if err != nil {
			t.Fatalf("post(/auth/login) error: %v", err)
		}
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
		}
		if !strings.Contains(recorder.Body.String(), `<html lang="en">`) || !strings.Contains(recorder.Body.String(), `Request timed out, please try again`) {
			t.Fatalf("unexpected response body: %s", recorder.Body.String())
		}
	})

	t.Run("without CSRF in welsh", func(t *testing.T) {
		h := newAuthControllerHarness(t)
		h.mockAuth.On("Login", mock.Anything, mock.Anything).Return()
		h.languageManagerMock.On("IsWelsh", mock.Anything).Return(true)
		h.languageManagerMock.On("LanguageError", authenticate.CSRF_ERR, mock.Anything).Return("Cais wedi dod i ben, triwch eto")

		recorder, err := h.post("/auth/login", "", nil)
		if err != nil {
			t.Fatalf("post(/auth/login) error: %v", err)
		}
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
		}
		if !strings.Contains(recorder.Body.String(), `<html lang="cy">`) || !strings.Contains(recorder.Body.String(), `Cais wedi dod i ben, triwch eto`) {
			t.Fatalf("unexpected response body: %s", recorder.Body.String())
		}
	})

	t.Run("with invalid CSRF logs mismatch", func(t *testing.T) {
		h := newAuthControllerHarness(t)
		h.mockAuth.On("Login", mock.Anything, mock.Anything).Return()
		h.languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
		h.languageManagerMock.On("LanguageError", authenticate.CSRF_ERR, mock.Anything).Return("Request timed out, please try again")

		recorder := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodPost, "/auth/login?_csrf=dalajksdqoosk", nil)
		if err != nil {
			t.Fatalf("http.NewRequest() error: %v", err)
		}
		req.RemoteAddr = "1.1.1.1"
		h.router.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
		}
		if !strings.Contains(recorder.Body.String(), `Request timed out, please try again`) {
			t.Fatalf("unexpected response body: %s", recorder.Body.String())
		}
		if h.observedLogs.Len() != 1 {
			t.Fatalf("log count = %d, want 1", h.observedLogs.Len())
		}
		entry := h.observedLogs.All()[0]
		if entry.Message != "CSRF mismatch" || entry.ContextMap()["SourceIP"] != "1.1.1.1" || entry.Level != zap.InfoLevel {
			t.Fatalf("unexpected log entry: %+v", entry)
		}
	})

	t.Run("with valid CSRF calls auth.Login", func(t *testing.T) {
		h := newAuthControllerHarness(t)
		h.mockAuth.On("Login", mock.Anything, mock.Anything).Return()
		h.languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
		csrfToken, cookieHeader := h.csrfTokenAndCookie(t)

		recorder, err := h.post(fmt.Sprintf("/auth/login?_csrf=%s", csrfToken), "", map[string]string{"Cookie": cookieHeader, "Content-Type": "application/x-www-form-urlencoded"})
		if err != nil {
			t.Fatalf("post(/auth/login) error: %v", err)
		}
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		h.mockAuth.AssertNumberOfCalls(t, "Login", 1)
	})

	t.Run("invalid UAC shows mode-specific message", func(t *testing.T) {
		t.Run("12-digit mode", func(t *testing.T) {
			h := newAuthControllerHarness(t)
			h.mockAuth.On("IsUAC16").Return(false)
			h.config.UACKind = "uac"
			h.mockAuth.On("Login", mock.Anything, mock.Anything).Return()
			h.languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
			csrfToken, cookieHeader := h.csrfTokenAndCookie(t)

			data := url.Values{"uac": []string{"123"}, "_csrf": []string{csrfToken}}
			recorder, err := h.post("/auth/login", data.Encode(), map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Cookie": cookieHeader})
			if err != nil {
				t.Fatalf("post(/auth/login) error: %v", err)
			}
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
			}
			if !strings.Contains(recorder.Body.String(), `Enter your 12-digit access code`) {
				t.Fatalf("unexpected response body: %s", recorder.Body.String())
			}
		})

		t.Run("16-character mode", func(t *testing.T) {
			h := newAuthControllerHarness(t)
			h.mockAuth.On("IsUAC16").Return(true)
			h.config.UACKind = "uac16"
			h.mockAuth.On("Login", mock.Anything, mock.Anything).Return()
			h.languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
			csrfToken, cookieHeader := h.csrfTokenAndCookie(t)

			data := url.Values{"uac": []string{"123"}, "_csrf": []string{csrfToken}}
			recorder, err := h.post("/auth/login", data.Encode(), map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Cookie": cookieHeader})
			if err != nil {
				t.Fatalf("post(/auth/login) error: %v", err)
			}
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
			}
			if !strings.Contains(recorder.Body.String(), `Enter your 16-character access code`) {
				t.Fatalf("unexpected response body: %s", recorder.Body.String())
			}
		})
	})
}

func TestAuthControllerLogoutEndpoint(t *testing.T) {
	h := newAuthControllerHarness(t)
	h.languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
	h.mockAuth.On("Logout", mock.Anything, mock.Anything).Return()

	recorder, err := h.get("/auth/logout")
	if err != nil {
		t.Fatalf("get(/auth/logout) error: %v", err)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	h.mockAuth.AssertNumberOfCalls(t, "Logout", 1)
}

func TestAuthControllerLoggedInEndpoint(t *testing.T) {
	t.Run("returns OK when session active", func(t *testing.T) {
		h := newAuthControllerHarness(t)
		h.mockAuth.On("HasSession", mock.Anything).Return(true, nil)
		recorder, err := h.get("/auth/logged-in")
		if err != nil {
			t.Fatalf("get(/auth/logged-in) error: %v", err)
		}
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
	})

	t.Run("returns unauthorized when no active session", func(t *testing.T) {
		h := newAuthControllerHarness(t)
		h.mockAuth.On("HasSession", mock.Anything).Return(false, nil)
		recorder, err := h.get("/auth/logged-in")
		if err != nil {
			t.Fatalf("get(/auth/logged-in) error: %v", err)
		}
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
		}
	})
}

func TestAuthControllerTimedOutEndpoint(t *testing.T) {
	runTimedOut := func(t *testing.T, welsh bool, timeoutValue interface{}) *httptest.ResponseRecorder {
		t.Helper()
		h := newAuthControllerHarness(t)
		h.languageManagerMock.On("IsWelsh", mock.Anything).Return(welsh)
		h.languageManagerMock.On("SetWelsh", mock.Anything, mock.Anything).Return()

		var sessionCookie string
		if timeoutValue != nil {
			h.router.GET("/set-timeout", func(c *gin.Context) {
				session := sessions.DefaultMany(c, "user_session")
				session.Set(authenticate.SESSION_TIMEOUT_KEY, timeoutValue)
				_ = session.Save()
				c.Status(http.StatusNoContent)
			})
			cookieRecorder, err := h.get("/set-timeout")
			if err != nil {
				t.Fatalf("get(/set-timeout) error: %v", err)
			}
			sessionCookie = cookieRecorder.Header().Get("Set-Cookie")
		}

		recorder := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "/auth/timed-out", nil)
		if err != nil {
			t.Fatalf("http.NewRequest() error: %v", err)
		}
		if sessionCookie != "" {
			req.Header.Set("Cookie", sessionCookie)
		}
		h.router.ServeHTTP(recorder, req)
		return recorder
	}

	t.Run("english response", func(t *testing.T) {
		recorder := runTimedOut(t, false, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		body := recorder.Body.String()
		if !strings.Contains(body, `Sorry, you need to sign in again`) ||
			!strings.Contains(body, `This is because you've been inactive for 15 minutes and your session has timed out to protect your information.`) ||
			!strings.Contains(body, `You need to <a href="/">sign back in</a> to continue your study.`) {
			t.Fatalf("unexpected body: %s", body)
		}
	})

	t.Run("welsh response", func(t *testing.T) {
		recorder := runTimedOut(t, true, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		body := recorder.Body.String()
		if !strings.Contains(body, `Mae'n ddrwg gennym, mae angen i chi fewngofnodi eto`) ||
			!strings.Contains(body, `Mae hyn oherwydd eich bod wedi bod yn anweithgar am 15 munud a bod eich sesiwn wedi cyrraedd y terfyn amser er mwyn diogelu eich gwybodaeth.`) ||
			!strings.Contains(body, `Bydd angen i chi <a href="/">fewngofnodi eto</a> i barhau â'ch astudiaeth.`) {
			t.Fatalf("unexpected body: %s", body)
		}
	})

	t.Run("malformed timeout falls back to default", func(t *testing.T) {
		recorder := runTimedOut(t, false, "15")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		if !strings.Contains(recorder.Body.String(), `you've been inactive for 15 minutes`) {
			t.Fatalf("unexpected body: %s", recorder.Body.String())
		}
	})
}
