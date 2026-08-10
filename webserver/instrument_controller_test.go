package webserver_test

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/authenticate/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	languageManagerMocks "github.com/ONSdigital/blaise-cawi-portal/languagemanager/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/webserver"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type TestResponseRecorder struct {
	*httptest.ResponseRecorder
	closeChannel chan bool
}

func (r *TestResponseRecorder) CloseNotify() <-chan bool {
	return r.closeChannel
}

func createTestResponseRecorder() *TestResponseRecorder {
	return &TestResponseRecorder{httptest.NewRecorder(), make(chan bool, 1)}
}

type ErrReader struct{ Error error }

func (e *ErrReader) Read([]byte) (int, error) {
	return 0, e.Error
}

type instrumentHarness struct {
	catiURL              string
	instrumentName       string
	caseID               string
	responseInfo         string
	router               *gin.Engine
	mockAuth             *mocks.AuthInterface
	mockJWTCrypto        *mocks.JWTCryptoInterface
	languageManagerMock  *languageManagerMocks.LanguageManagerInterface
	instrumentController *webserver.InstrumentController
	observedLogs         *observer.ObservedLogs
}

func newInstrumentHarness(t *testing.T) *instrumentHarness {
	t.Helper()

	h := &instrumentHarness{
		catiURL:             "http://localhost",
		instrumentName:      "foobar",
		caseID:              "fizzbuzz",
		responseInfo:        "<html><head></head><body></body></html>",
		mockAuth:            &mocks.AuthInterface{},
		mockJWTCrypto:       &mocks.JWTCryptoInterface{},
		languageManagerMock: &languageManagerMocks.LanguageManagerInterface{},
	}

	h.router = gin.Default()
	h.router.SetFuncMap(template.FuncMap{"WrapWelsh": webserver.WrapWelsh})
	h.router.LoadHTMLGlob("../templates/*")
	store := cookie.NewStore([]byte("secret"))
	h.router.Use(sessions.SessionsMany([]string{"session", "user_session", "session_validation", "language_session"}, store))

	var observedZapCore zapcore.Core
	observedZapCore, h.observedLogs = observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore)
	_ = observedLogger.Sync()

	h.instrumentController = &webserver.InstrumentController{
		CatiURL:         h.catiURL,
		HttpClient:      &http.Client{Timeout: 3 * time.Minute},
		Auth:            h.mockAuth,
		JWTCrypto:       h.mockJWTCrypto,
		LanguageManager: h.languageManagerMock,
		Logger:          observedLogger,
	}
	h.instrumentController.AddRoutes(h.router)

	h.mockAuth.On("RefreshToken", mock.Anything, mock.Anything, mock.Anything).Return()

	httpmock.Activate()
	t.Cleanup(httpmock.DeactivateAndReset)

	return h
}

func (h *instrumentHarness) get(t *testing.T, path string) *TestResponseRecorder {
	t.Helper()
	recorder := createTestResponseRecorder()
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error: %v", err)
	}
	h.router.ServeHTTP(recorder, req)
	return recorder
}

func (h *instrumentHarness) post(t *testing.T, path string, body io.Reader) *TestResponseRecorder {
	t.Helper()
	recorder := createTestResponseRecorder()
	req, err := http.NewRequest(http.MethodPost, path, body)
	if err != nil {
		t.Fatalf("http.NewRequest() error: %v", err)
	}
	h.router.ServeHTTP(recorder, req)
	return recorder
}

func authedClaims(instrumentName, caseID string) *authenticate.UACClaims {
	return &authenticate.UACClaims{UACInfo: busapi.UACInfo{InstrumentName: instrumentName, CaseID: caseID}}
}

func TestInstrumentOpenCase(t *testing.T) {
	t.Run("injects script for valid instrument", func(t *testing.T) {
		h := newInstrumentHarness(t)
		h.languageManagerMock.On("IsWelsh", mock.Anything).Return(false)

		mockResponse := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader(h.responseInfo))}
		httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/default.aspx", h.catiURL, h.instrumentName), httpmock.ResponderFromResponse(mockResponse))

		h.mockAuth.On("AuthenticatedWithUAC", mock.Anything).Return()
		h.mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(authedClaims(h.instrumentName, h.caseID), nil)

		recorder := h.get(t, fmt.Sprintf("/%s/", h.instrumentName))
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		expected := `<html><head></head><body><script src="/assets/js/check-session.js"></script></body></html>`
		if recorder.Body.String() != expected {
			t.Fatalf("body = %q, want %q", recorder.Body.String(), expected)
		}
	})

	t.Run("falls back to instrument root when default.aspx missing", func(t *testing.T) {
		h := newInstrumentHarness(t)
		h.languageManagerMock.On("IsWelsh", mock.Anything).Return(false)

		mockResponse := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader(h.responseInfo))}
		httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/default.aspx", h.catiURL, h.instrumentName), httpmock.NewStringResponder(http.StatusNotFound, "not found"))
		httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/", h.catiURL, h.instrumentName), httpmock.ResponderFromResponse(mockResponse))

		h.mockAuth.On("AuthenticatedWithUAC", mock.Anything).Return()
		h.mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(authedClaims(h.instrumentName, h.caseID), nil)

		recorder := h.get(t, fmt.Sprintf("/%s/", h.instrumentName))
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		expected := `<html><head></head><body><script src="/assets/js/check-session.js"></script></body></html>`
		if recorder.Body.String() != expected {
			t.Fatalf("body = %q, want %q", recorder.Body.String(), expected)
		}
	})

	t.Run("forbidden for different instrument in welsh", func(t *testing.T) {
		h := newInstrumentHarness(t)
		h.languageManagerMock.On("IsWelsh", mock.Anything).Return(true)

		httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/default.aspx", h.catiURL, h.instrumentName), httpmock.NewStringResponder(http.StatusOK, h.responseInfo))
		h.mockAuth.On("AuthenticatedWithUAC", mock.Anything).Return()
		h.mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(authedClaims(h.instrumentName, h.caseID), nil)

		recorder := h.get(t, "/fwibble/")
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
		}
		if !strings.Contains(recorder.Body.String(), `I fynd i'r dudalen hon, bydd angen i chi .<a href="/">roi eich cod mynediad eto</a>.`) {
			t.Fatalf("unexpected body: %s", recorder.Body.String())
		}

		if h.observedLogs.Len() != 1 {
			t.Fatalf("log count = %d, want 1", h.observedLogs.Len())
		}
		entry := h.observedLogs.All()[0]
		if entry.Message != "Not authenticated for instrument" || entry.ContextMap()["AuthedCaseIDFingerprint"] != "4ee36d70199f" || entry.ContextMap()["AuthedInstrumentName"] != h.instrumentName || entry.ContextMap()["InstrumentName"] != "fwibble" || entry.Level != zap.InfoLevel {
			t.Fatalf("unexpected log entry: %+v", entry)
		}
	})

	t.Run("forbidden for different instrument in english", func(t *testing.T) {
		h := newInstrumentHarness(t)
		h.languageManagerMock.On("IsWelsh", mock.Anything).Return(false)

		httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/default.aspx", h.catiURL, h.instrumentName), httpmock.NewStringResponder(http.StatusOK, h.responseInfo))
		h.mockAuth.On("AuthenticatedWithUAC", mock.Anything).Return()
		h.mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(authedClaims(h.instrumentName, h.caseID), nil)

		recorder := h.get(t, "/fwibble/")
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
		}
		if !strings.Contains(recorder.Body.String(), `To access this page you need to <a href="/">re-enter your access code</a>`) {
			t.Fatalf("unexpected body: %s", recorder.Body.String())
		}
	})

	t.Run("decrypt JWT failure logs and calls NotAuthWithError", func(t *testing.T) {
		h := newInstrumentHarness(t)
		h.languageManagerMock.On("LanguageError", mock.Anything, mock.Anything).Return("We were unable to process your request, please try again")
		h.mockAuth.On("AuthenticatedWithUAC", mock.Anything).Return()
		h.mockAuth.On("NotAuthWithError", mock.Anything, mock.Anything).Return()
		h.mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(nil, errors.New("No JWT"))

		_ = h.get(t, fmt.Sprintf("/%s/", h.instrumentName))
		h.mockAuth.AssertNumberOfCalls(t, "NotAuthWithError", 1)

		if h.observedLogs.Len() != 1 {
			t.Fatalf("log count = %d, want 1", h.observedLogs.Len())
		}
		entry := h.observedLogs.All()[0]
		if entry.Message != "Error decrypting JWT" || entry.ContextMap()["error"] != "No JWT" || entry.Level != zap.ErrorLevel {
			t.Fatalf("unexpected log entry: %+v", entry)
		}
	})

	t.Run("non-200 from blaise returns internal server error", func(t *testing.T) {
		h := newInstrumentHarness(t)
		h.languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
		httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/default.aspx", h.catiURL, h.instrumentName), httpmock.NewJsonResponderOrPanic(http.StatusInternalServerError, "Sad face"))

		h.mockAuth.On("AuthenticatedWithUAC", mock.Anything).Return()
		h.mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(authedClaims(h.instrumentName, h.caseID), nil)

		recorder := h.get(t, fmt.Sprintf("/%s/", h.instrumentName))
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
		}
		if !strings.Contains(recorder.Body.String(), "Sorry, there is a problem with the service") {
			t.Fatalf("unexpected body: %s", recorder.Body.String())
		}

		if h.observedLogs.Len() != 1 {
			t.Fatalf("log count = %d, want 1", h.observedLogs.Len())
		}
		entry := h.observedLogs.All()[0]
		if entry.Message != "Error launching blaise study, invalid status code" || entry.ContextMap()["AuthedCaseIDFingerprint"] != "4ee36d70199f" || entry.ContextMap()["AuthedInstrumentName"] != h.instrumentName || entry.ContextMap()["RespStatusCode"] != int64(500) || entry.ContextMap()["RespBodyBytes"] != int64(10) || entry.Level != zap.ErrorLevel {
			t.Fatalf("unexpected log entry: %+v", entry)
		}
	})
}

func TestInstrumentProxyGetRequests(t *testing.T) {
	t.Run("long URL resource is proxied", func(t *testing.T) {
		h := newInstrumentHarness(t)
		httpmock.RegisterResponder("GET", fmt.Sprintf("%s/%s/fwibble/dwibble/qwibble", h.catiURL, h.instrumentName), httpmock.NewStringResponder(http.StatusOK, h.responseInfo))

		h.mockAuth.On("AuthenticatedWithUAC", mock.Anything).Return()
		h.mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(authedClaims(h.instrumentName, h.caseID), nil)

		recorder := createTestResponseRecorder()
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/%s/fwibble/dwibble/qwibble", h.instrumentName), nil)
		if err != nil {
			t.Fatalf("http.NewRequest() error: %v", err)
		}
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Connection", "foobar")
		h.router.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), h.responseInfo) {
			t.Fatalf("unexpected response: code=%d body=%s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("short URL resource is proxied", func(t *testing.T) {
		h := newInstrumentHarness(t)
		httpmock.RegisterResponder("GET", fmt.Sprintf("%s/%s/fwibble", h.catiURL, h.instrumentName), httpmock.NewStringResponder(http.StatusOK, h.responseInfo))

		h.mockAuth.On("AuthenticatedWithUAC", mock.Anything).Return()
		h.mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(authedClaims(h.instrumentName, h.caseID), nil)

		recorder := h.get(t, fmt.Sprintf("/%s/fwibble", h.instrumentName))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), h.responseInfo) {
			t.Fatalf("unexpected response: code=%d body=%s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("different instrument request is forbidden", func(t *testing.T) {
		h := newInstrumentHarness(t)
		h.languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
		httpmock.RegisterResponder("GET", fmt.Sprintf("%s/%s/fwibble", h.catiURL, "notMyInstrument"), httpmock.NewStringResponder(http.StatusOK, h.responseInfo))

		h.mockAuth.On("AuthenticatedWithUAC", mock.Anything).Return()
		h.mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(authedClaims(h.instrumentName, h.caseID), nil)

		recorder := h.get(t, "/notMyInstrument/fwibble")
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
		}
		if !strings.Contains(recorder.Body.String(), `To access this page you need to <a href="/">re-enter your access code</a>`) {
			t.Fatalf("unexpected body: %s", recorder.Body.String())
		}
	})
}

func TestInstrumentProxyPostRequests(t *testing.T) {
	t.Run("generic post is proxied", func(t *testing.T) {
		h := newInstrumentHarness(t)
		httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/fwibble", h.catiURL, h.instrumentName), httpmock.NewStringResponder(http.StatusOK, h.responseInfo))

		h.mockAuth.On("AuthenticatedWithUAC", mock.Anything).Return()
		h.mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(authedClaims(h.instrumentName, h.caseID), nil)

		recorder := h.post(t, fmt.Sprintf("/%s/fwibble", h.instrumentName), bytes.NewReader([]byte(`{"foo":"bar"}`)))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), h.responseInfo) {
			t.Fatalf("unexpected response: code=%d body=%s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("start interview for authorised case succeeds", func(t *testing.T) {
		h := newInstrumentHarness(t)
		httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/api/application/start_interview", h.catiURL, h.instrumentName), httpmock.NewStringResponder(http.StatusOK, h.responseInfo))

		h.mockAuth.On("AuthenticatedWithUAC", mock.Anything).Return()
		h.mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(authedClaims(h.instrumentName, h.caseID), nil)

		requestBody := bytes.NewReader([]byte(fmt.Sprintf(`{"RuntimeParameters":{"KeyValue":"%s","Mode":"CAWI"}}`, h.caseID)))
		recorder := h.post(t, fmt.Sprintf("/%s/api/application/start_interview", h.instrumentName), requestBody)
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), h.responseInfo) {
			t.Fatalf("unexpected response: code=%d body=%s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("start interview for unauthorised case is forbidden", func(t *testing.T) {
		h := newInstrumentHarness(t)
		h.languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
		httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/api/application/start_interview", h.catiURL, h.instrumentName), httpmock.NewStringResponder(http.StatusOK, h.responseInfo))

		h.mockAuth.On("AuthenticatedWithUAC", mock.Anything).Return()
		h.mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(authedClaims(h.instrumentName, h.caseID), nil)

		requestBody := bytes.NewReader([]byte(`{"RuntimeParameters":{"KeyValue":"notMyCaseID","Mode":"CAWI"}}`))
		recorder := h.post(t, fmt.Sprintf("/%s/api/application/start_interview", h.instrumentName), requestBody)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
		}
		if !strings.Contains(recorder.Body.String(), `To access this page you need to <a href="/">re-enter your access code</a>`) {
			t.Fatalf("unexpected body: %s", recorder.Body.String())
		}

		if h.observedLogs.Len() != 1 {
			t.Fatalf("log count = %d, want 1", h.observedLogs.Len())
		}
		entry := h.observedLogs.All()[0]
		if entry.Message != "Not authenticated to start interview for case" || entry.ContextMap()["AuthedCaseIDFingerprint"] != "4ee36d70199f" || entry.ContextMap()["AuthedInstrumentName"] != h.instrumentName || entry.ContextMap()["CaseIDFingerprint"] != "0ec2a64e939a" || entry.Level != zap.InfoLevel {
			t.Fatalf("unexpected log entry: %+v", entry)
		}
	})
}

func TestInstrumentLogoutRoute(t *testing.T) {
	mockAuth := &mocks.AuthInterface{}
	instrumentController := &webserver.InstrumentController{Auth: mockAuth}
	router := gin.Default()
	store := cookie.NewStore([]byte("secret"))
	router.Use(sessions.SessionsMany([]string{"session", "user_session", "session_validation"}, store))
	router.SetFuncMap(template.FuncMap{"WrapWelsh": webserver.WrapWelsh})
	router.LoadHTMLGlob("../templates/*")
	instrumentController.AddRoutes(router)

	mockAuth.On("Logout", mock.Anything, mock.Anything).Return()

	recorder := createTestResponseRecorder()
	req, err := http.NewRequest(http.MethodGet, "/foobar/logout", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error: %v", err)
	}
	router.ServeHTTP(recorder, req)

	mockAuth.AssertNumberOfCalls(t, "Logout", 1)
}

func TestInstrumentProxyInternals(t *testing.T) {
	t.Run("invalid proxy URL returns internal server error", func(t *testing.T) {
		h := newInstrumentHarness(t)
		h.instrumentController.CatiURL = "://bad-url"
		h.languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
		h.mockAuth.On("AuthenticatedWithUAC", mock.Anything).Return()
		h.mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(authedClaims("foobar", "fizzbuzz"), nil)

		recorder := h.get(t, "/foobar/resources")
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
		}
	})

	t.Run("malformed start interview JSON returns internal server error", func(t *testing.T) {
		h := newInstrumentHarness(t)
		h.languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
		h.mockAuth.On("AuthenticatedWithUAC", mock.Anything).Return()
		h.mockAuth.On("RefreshToken", mock.Anything, mock.Anything, mock.Anything).Return()
		h.mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(authedClaims("foobar", "fizzbuzz"), nil)

		recorder := h.post(t, "/foobar/api/application/start_interview", strings.NewReader("{invalid"))
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
		}
	})

	t.Run("unreadable start interview body returns internal server error", func(t *testing.T) {
		h := newInstrumentHarness(t)
		h.languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
		h.mockAuth.On("AuthenticatedWithUAC", mock.Anything).Return()
		h.mockAuth.On("RefreshToken", mock.Anything, mock.Anything, mock.Anything).Return()
		h.mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(authedClaims("foobar", "fizzbuzz"), nil)

		recorder := createTestResponseRecorder()
		req, err := http.NewRequest(http.MethodPost, "/foobar/api/application/start_interview", nil)
		if err != nil {
			t.Fatalf("http.NewRequest() error: %v", err)
		}
		req.Body = io.NopCloser(&ErrReader{Error: errors.New("read failed")})
		h.router.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
		}
	})
}
