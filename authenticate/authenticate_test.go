package authenticate_test

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
	"github.com/ONSdigital/blaise-cawi-portal/blaiserestapi"
	restapimocks "github.com/ONSdigital/blaise-cawi-portal/blaiserestapi/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	busmocks "github.com/ONSdigital/blaise-cawi-portal/busapi/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/csrf"
	languagemocks "github.com/ONSdigital/blaise-cawi-portal/languagemanager/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/sessionkeys"
	"github.com/ONSdigital/blaise-cawi-portal/webserver"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type loginHarness struct {
	auth           *authenticate.Auth
	router         *gin.Engine
	session        sessions.Session
	logs           *observer.ObservedLogs
	languageMock   *languagemocks.LanguageManagerInterface
	csrfManager    *csrf.DefaultCSRFManager
	jwtCrypto      *authenticate.JWTCrypto
	observedLogger *zap.Logger
}

func newLoginHarness(t *testing.T, welsh bool) *loginHarness {
	t.Helper()

	var observedZapCore zapcore.Core
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore)

	languageManagerMock := &languagemocks.LanguageManagerInterface{}
	languageManagerMock.On("IsWelsh", mock.Anything).Return(welsh)
	languageManagerMock.On("LanguageError", authenticate.NOT_RECOGNISED_ERR, mock.Anything).Return("Access code not recognised. Enter the code again")
	languageManagerMock.On("LanguageError", authenticate.INTERNAL_SERVER_ERR, mock.Anything).Return("We were unable to process your request, please try again")

	jwtCrypto := &authenticate.JWTCrypto{JWTSecret: "hello"}
	csrfManager := &csrf.DefaultCSRFManager{Secret: "fwibble", SessionName: sessionkeys.SessionName}
	auth := &authenticate.Auth{
		JWTCrypto:       jwtCrypto,
		Logger:          observedLogger,
		CSRFManager:     csrfManager,
		LanguageManager: languageManagerMock,
	}

	router := gin.Default()
	router.SetFuncMap(template.FuncMap{"WrapWelsh": webserver.WrapWelsh})
	router.LoadHTMLGlob("../templates/*")
	store := cookie.NewStore([]byte("secret"))
	router.Use(sessions.SessionsMany([]string{sessionkeys.SessionName, sessionkeys.UserSessionName, sessionkeys.SessionValidationName, sessionkeys.LanguageSessionName}, store))

	h := &loginHarness{
		auth:           auth,
		router:         router,
		logs:           observedLogs,
		languageMock:   languageManagerMock,
		csrfManager:    csrfManager,
		jwtCrypto:      jwtCrypto,
		observedLogger: observedLogger,
	}

	router.POST("/login", func(c *gin.Context) {
		h.session = sessions.DefaultMany(c, sessionkeys.UserSessionName)
		h.auth.Login(c, h.session)
	})

	return h
}

func (h *loginHarness) postLogin(t *testing.T, uacValue, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	data := url.Values{"uac": []string{uacValue}}
	req, err := http.NewRequest(http.MethodPost, "/login", strings.NewReader(data.Encode()))
	require.NoError(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}

	h.router.ServeHTTP(recorder, req)
	return recorder
}

func TestLogin(t *testing.T) {
	const (
		shortUAC    = "22222"
		longUAC     = "11112222333344445555"
		spacedUAC   = "1234 5678 9012"
		spacedUAC16 = "bcdf 5678 ghjk 2345"
		validUAC    = "123456789012"
		validUAC16  = "bcdf5678ghjk2345"
	)

	t.Run("when instrument is not installed", func(t *testing.T) {
		h := newLoginHarness(t, false)
		h.auth.UACKind = "uac"

		mockBusAPI := &busmocks.BUSAPIInterface{}
		mockBusAPI.On("GetUACInfo", mock.Anything, validUAC).Once().Return(busapi.UACInfo{InstrumentName: "foo", CaseID: "bar"}, nil)
		h.auth.BUSAPI = mockBusAPI

		mockRestAPI := &restapimocks.BlaiseRestAPIInterface{}
		mockRestAPI.On("GetInstrumentSettings", mock.Anything, mock.Anything).Return(blaiserestapi.InstrumentSettings{}, blaiserestapi.InstrumentNotFoundError)
		h.auth.BlaiseRestAPI = mockRestAPI

		recorder := h.postLogin(t, validUAC, "")
		assert.Equal(t, http.StatusOK, recorder.Code)
		body := recorder.Body.String()
		assert.Contains(t, body, "The study is currently unavailable")
		assert.Contains(t, body, "Please try again later or contact our Survey Enquiry Line on 0800 085 7376 for help.")
		assert.Contains(t, body, "Any answers you have provided in previous sessions have been logged securely and confidentially. They will only be used for the purposes of this research.")

		require.Equal(t, 1, h.logs.Len())
		entry := h.logs.All()[0]
		assert.Equal(t, "Failed auth", entry.Message)
		assert.Equal(t, "Instrument not installed", entry.ContextMap()["Reason"])
		assert.Equal(t, "This can happen if a UAC for a non-Blaise 5 survey has been entered", entry.ContextMap()["Notes"])
		assert.Equal(t, "foo", entry.ContextMap()["InstrumentName"])
		assert.Equal(t, "fcde2b2edba5", entry.ContextMap()["CaseIDFingerprint"])
		assert.Equal(t, zap.WarnLevel, entry.Level)
	})

	t.Run("when instrument settings does not error", func(t *testing.T) {
		t.Run("correct length but invalid UAC", func(t *testing.T) {
			h := newLoginHarness(t, false)
			h.auth.UACKind = "uac"

			mockRestAPI := &restapimocks.BlaiseRestAPIInterface{}
			mockRestAPI.On("GetInstrumentSettings", mock.Anything, mock.Anything).Return(blaiserestapi.InstrumentSettings{}, nil)
			h.auth.BlaiseRestAPI = mockRestAPI

			mockBusAPI := &busmocks.BUSAPIInterface{}
			mockBusAPI.On("GetUACInfo", mock.Anything, validUAC).Once().Return(busapi.UACInfo{InstrumentName: "", CaseID: "bar"}, nil)
			h.auth.BUSAPI = mockBusAPI

			recorder := h.postLogin(t, validUAC, "1.1.1.1")
			assert.Equal(t, http.StatusUnauthorized, recorder.Code)
			assert.NotEmpty(t, recorder.Result().Cookies())
			assert.Nil(t, h.session.Get(authenticate.JWT_TOKEN_KEY))
			assert.Contains(t, recorder.Body.String(), "Access code not recognised. Enter the code again")

			require.Equal(t, 1, h.logs.Len())
			entry := h.logs.All()[0]
			assert.Equal(t, "Failed auth", entry.Message)
			assert.Equal(t, "1.1.1.1", entry.ContextMap()["SourceIP"])
			assert.Equal(t, "Access code not recognised", entry.ContextMap()["Reason"])
			assert.Equal(t, "", entry.ContextMap()["InstrumentName"])
			assert.Equal(t, "fcde2b2edba5", entry.ContextMap()["CaseIDFingerprint"])
			assert.Nil(t, entry.ContextMap()["error"])
			assert.Equal(t, zap.InfoLevel, entry.Level)
		})

		t.Run("correct length but BUS API errors", func(t *testing.T) {
			h := newLoginHarness(t, false)
			h.auth.UACKind = "uac"

			mockBusAPI := &busmocks.BUSAPIInterface{}
			mockBusAPI.On("GetUACInfo", mock.Anything, validUAC).Once().Return(busapi.UACInfo{}, fmt.Errorf("bus unavailable"))
			h.auth.BUSAPI = mockBusAPI

			recorder := h.postLogin(t, validUAC, "1.1.1.1")
			assert.Equal(t, http.StatusUnauthorized, recorder.Code)
			assert.NotEmpty(t, recorder.Result().Cookies())
			assert.Nil(t, h.session.Get(authenticate.JWT_TOKEN_KEY))
			assert.Contains(t, recorder.Body.String(), "We were unable to process your request, please try again")

			require.Equal(t, 1, h.logs.Len())
			entry := h.logs.All()[0]
			assert.Equal(t, "Failed auth", entry.Message)
			assert.Equal(t, "1.1.1.1", entry.ContextMap()["SourceIP"])
			assert.Equal(t, "Error retrieving UAC information", entry.ContextMap()["Reason"])
			assert.Nil(t, entry.ContextMap()["InstrumentName"])
			assert.Nil(t, entry.ContextMap()["CaseIDFingerprint"])
			assert.Equal(t, "bus unavailable", entry.ContextMap()["error"])
			assert.Equal(t, zap.ErrorLevel, entry.Level)
		})

		t.Run("valid UAC code", func(t *testing.T) {
			t.Run("12 digit UAC", func(t *testing.T) {
				h := newLoginHarness(t, false)
				h.auth.UACKind = "uac"

				mockRestAPI := &restapimocks.BlaiseRestAPIInterface{}
				mockRestAPI.On("GetInstrumentSettings", mock.Anything, mock.Anything).Return(blaiserestapi.InstrumentSettings{}, nil)
				h.auth.BlaiseRestAPI = mockRestAPI

				mockBusAPI := &busmocks.BUSAPIInterface{}
				mockBusAPI.On("GetUACInfo", mock.Anything, validUAC).Once().Return(busapi.UACInfo{InstrumentName: "foo", CaseID: "bar"}, nil)
				h.auth.BUSAPI = mockBusAPI

				recorder := h.postLogin(t, validUAC, "")
				assert.Equal(t, http.StatusFound, recorder.Code)
				assert.Equal(t, []string{"/foo/"}, recorder.Header()["Location"])
				assert.NotEmpty(t, recorder.Result().Cookies())

				decryptedToken, err := h.auth.JWTCrypto.DecryptJWT(h.session.Get(authenticate.JWT_TOKEN_KEY))
				require.NoError(t, err)
				require.NotNil(t, decryptedToken)
				assert.Equal(t, validUAC, decryptedToken.UAC)
				assert.Equal(t, "foo", decryptedToken.UACInfo.InstrumentName)
				assert.Equal(t, "bar", decryptedToken.UACInfo.CaseID)
				assert.Equal(t, 15, h.session.Get(authenticate.SESSION_TIMEOUT_KEY).(int))
			})

			t.Run("16 character UAC", func(t *testing.T) {
				h := newLoginHarness(t, false)
				h.auth.UACKind = "uac16"

				mockRestAPI := &restapimocks.BlaiseRestAPIInterface{}
				mockRestAPI.On("GetInstrumentSettings", mock.Anything, mock.Anything).Return(blaiserestapi.InstrumentSettings{}, nil)
				h.auth.BlaiseRestAPI = mockRestAPI

				mockBusAPI := &busmocks.BUSAPIInterface{}
				mockBusAPI.On("GetUACInfo", mock.Anything, validUAC16).Once().Return(busapi.UACInfo{InstrumentName: "foo", CaseID: "bar"}, nil)
				h.auth.BUSAPI = mockBusAPI

				recorder := h.postLogin(t, validUAC16, "")
				assert.Equal(t, http.StatusFound, recorder.Code)
				assert.Equal(t, []string{"/foo/"}, recorder.Header()["Location"])
				assert.NotEmpty(t, recorder.Result().Cookies())

				decryptedToken, err := h.auth.JWTCrypto.DecryptJWT(h.session.Get(authenticate.JWT_TOKEN_KEY))
				require.NoError(t, err)
				require.NotNil(t, decryptedToken)
				assert.Equal(t, validUAC16, decryptedToken.UAC)
				assert.Equal(t, "foo", decryptedToken.UACInfo.InstrumentName)
				assert.Equal(t, "bar", decryptedToken.UACInfo.CaseID)
				assert.Equal(t, 15, h.session.Get(authenticate.SESSION_TIMEOUT_KEY).(int))
			})
		})

		t.Run("valid UAC code containing whitespace", func(t *testing.T) {
			t.Run("12 digit UAC", func(t *testing.T) {
				h := newLoginHarness(t, false)
				h.auth.UACKind = "uac"

				mockRestAPI := &restapimocks.BlaiseRestAPIInterface{}
				mockRestAPI.On("GetInstrumentSettings", mock.Anything, mock.Anything).Return(blaiserestapi.InstrumentSettings{}, nil)
				h.auth.BlaiseRestAPI = mockRestAPI

				mockBusAPI := &busmocks.BUSAPIInterface{}
				mockBusAPI.On("GetUACInfo", mock.Anything, validUAC).Once().Return(busapi.UACInfo{InstrumentName: "foo", CaseID: "bar"}, nil)
				h.auth.BUSAPI = mockBusAPI

				recorder := h.postLogin(t, spacedUAC, "")
				assert.Equal(t, http.StatusFound, recorder.Code)
				assert.Equal(t, []string{"/foo/"}, recorder.Header()["Location"])
				assert.NotEmpty(t, recorder.Result().Cookies())

				decryptedToken, err := h.auth.JWTCrypto.DecryptJWT(h.session.Get(authenticate.JWT_TOKEN_KEY))
				require.NoError(t, err)
				require.NotNil(t, decryptedToken)
				assert.Equal(t, validUAC, decryptedToken.UAC)
				assert.Equal(t, "foo", decryptedToken.UACInfo.InstrumentName)
				assert.Equal(t, "bar", decryptedToken.UACInfo.CaseID)

				require.Equal(t, 1, h.logs.Len())
				assert.Equal(t, "Successful auth with questionnaire: foo, case ID: bar", h.logs.All()[0].Message)
				assert.NotContains(t, h.logs.All()[0].Message, validUAC)
				assert.NotContains(t, fmt.Sprint(h.logs.All()[0].ContextMap()), validUAC)
			})

			t.Run("16 character UAC", func(t *testing.T) {
				h := newLoginHarness(t, false)
				h.auth.UACKind = "uac16"

				mockRestAPI := &restapimocks.BlaiseRestAPIInterface{}
				mockRestAPI.On("GetInstrumentSettings", mock.Anything, mock.Anything).Return(blaiserestapi.InstrumentSettings{}, nil)
				h.auth.BlaiseRestAPI = mockRestAPI

				mockBusAPI := &busmocks.BUSAPIInterface{}
				mockBusAPI.On("GetUACInfo", mock.Anything, validUAC16).Once().Return(busapi.UACInfo{InstrumentName: "foo", CaseID: "bar"}, nil)
				h.auth.BUSAPI = mockBusAPI

				recorder := h.postLogin(t, spacedUAC16, "")
				assert.Equal(t, http.StatusFound, recorder.Code)
				assert.Equal(t, []string{"/foo/"}, recorder.Header()["Location"])
				assert.NotEmpty(t, recorder.Result().Cookies())

				decryptedToken, err := h.auth.JWTCrypto.DecryptJWT(h.session.Get(authenticate.JWT_TOKEN_KEY))
				require.NoError(t, err)
				require.NotNil(t, decryptedToken)
				assert.Equal(t, validUAC16, decryptedToken.UAC)
				assert.Equal(t, "foo", decryptedToken.UACInfo.InstrumentName)
				assert.Equal(t, "bar", decryptedToken.UACInfo.CaseID)
			})
		})

		t.Run("short UAC code", func(t *testing.T) {
			t.Run("12 digit mode", func(t *testing.T) {
				h := newLoginHarness(t, false)
				h.auth.UACKind = "uac"
				recorder := h.postLogin(t, shortUAC, "")
				assert.Equal(t, http.StatusUnauthorized, recorder.Code)
				assert.NotEmpty(t, recorder.Result().Cookies())
				assert.Nil(t, h.session.Get(authenticate.JWT_TOKEN_KEY))
				assert.Contains(t, recorder.Body.String(), "Enter your 12-digit access code")
			})

			t.Run("16 character mode", func(t *testing.T) {
				h := newLoginHarness(t, false)
				h.auth.UACKind = "uac16"
				recorder := h.postLogin(t, shortUAC, "")
				assert.Equal(t, http.StatusUnauthorized, recorder.Code)
				assert.NotEmpty(t, recorder.Result().Cookies())
				assert.Nil(t, h.session.Get(authenticate.JWT_TOKEN_KEY))
				assert.Contains(t, recorder.Body.String(), "Enter your 16-character access code")
			})
		})

		t.Run("long UAC code", func(t *testing.T) {
			t.Run("12 digit mode", func(t *testing.T) {
				h := newLoginHarness(t, false)
				h.auth.UACKind = "uac"
				recorder := h.postLogin(t, longUAC, "1.1.1.1")
				assert.Equal(t, http.StatusUnauthorized, recorder.Code)
				assert.NotEmpty(t, recorder.Result().Cookies())
				assert.Nil(t, h.session.Get(authenticate.JWT_TOKEN_KEY))
				assert.Contains(t, recorder.Body.String(), "Enter your 12-digit access code")

				require.Equal(t, 1, h.logs.Len())
				entry := h.logs.All()[0]
				assert.Equal(t, "Failed auth", entry.Message)
				assert.Equal(t, "1.1.1.1", entry.ContextMap()["SourceIP"])
				assert.Equal(t, "Invalid UAC length", entry.ContextMap()["Reason"])
				assert.Equal(t, int64(12), entry.ContextMap()["UACLength"])
				assert.Equal(t, zap.InfoLevel, entry.Level)
			})

			t.Run("16 character mode", func(t *testing.T) {
				h := newLoginHarness(t, false)
				h.auth.UACKind = "uac16"
				recorder := h.postLogin(t, longUAC, "1.1.1.1")
				assert.Equal(t, http.StatusUnauthorized, recorder.Code)
				assert.NotEmpty(t, recorder.Result().Cookies())
				assert.Nil(t, h.session.Get(authenticate.JWT_TOKEN_KEY))
				assert.Contains(t, recorder.Body.String(), "Enter your 16-character access code")

				require.Equal(t, 1, h.logs.Len())
				entry := h.logs.All()[0]
				assert.Equal(t, "Failed auth", entry.Message)
				assert.Equal(t, "1.1.1.1", entry.ContextMap()["SourceIP"])
				assert.Equal(t, "Invalid UAC length", entry.ContextMap()["Reason"])
				assert.Equal(t, int64(16), entry.ContextMap()["UACLength"])
				assert.Equal(t, zap.InfoLevel, entry.Level)
			})
		})

		t.Run("blank UAC code", func(t *testing.T) {
			t.Run("12 digit mode", func(t *testing.T) {
				h := newLoginHarness(t, false)
				h.auth.UACKind = "uac"
				recorder := h.postLogin(t, "", "1.1.1.1")
				assert.Equal(t, http.StatusUnauthorized, recorder.Code)
				assert.NotEmpty(t, recorder.Result().Cookies())
				assert.Nil(t, h.session.Get(authenticate.JWT_TOKEN_KEY))
				assert.Contains(t, recorder.Body.String(), "Enter your 12-digit access code")

				require.Equal(t, 1, h.logs.Len())
				entry := h.logs.All()[0]
				assert.Equal(t, "Failed auth", entry.Message)
				assert.Equal(t, "1.1.1.1", entry.ContextMap()["SourceIP"])
				assert.Equal(t, "Blank UAC", entry.ContextMap()["Reason"])
				assert.Equal(t, zap.InfoLevel, entry.Level)
			})

			t.Run("16 character mode", func(t *testing.T) {
				h := newLoginHarness(t, false)
				h.auth.UACKind = "uac16"
				recorder := h.postLogin(t, "", "1.1.1.1")
				assert.Equal(t, http.StatusUnauthorized, recorder.Code)
				assert.NotEmpty(t, recorder.Result().Cookies())
				assert.Nil(t, h.session.Get(authenticate.JWT_TOKEN_KEY))
				assert.Contains(t, recorder.Body.String(), "Enter your 16-character access code")

				require.Equal(t, 1, h.logs.Len())
				entry := h.logs.All()[0]
				assert.Equal(t, "Failed auth", entry.Message)
				assert.Equal(t, "1.1.1.1", entry.ContextMap()["SourceIP"])
				assert.Equal(t, "Blank UAC", entry.ContextMap()["Reason"])
				assert.Equal(t, zap.InfoLevel, entry.Level)
			})
		})
	})
}

func TestLogout(t *testing.T) {
	languageManagerMock := &languagemocks.LanguageManagerInterface{}
	languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
	auth := &authenticate.Auth{
		CSRFManager:     &csrf.DefaultCSRFManager{Secret: "fwibble", SessionName: sessionkeys.SessionName},
		LanguageManager: languageManagerMock,
	}

	router := gin.Default()
	router.SetFuncMap(template.FuncMap{"WrapWelsh": webserver.WrapWelsh})
	router.LoadHTMLGlob("../templates/*")
	store := cookie.NewStore([]byte("secret"))
	router.Use(sessions.SessionsMany([]string{sessionkeys.SessionName, sessionkeys.UserSessionName, sessionkeys.SessionValidationName, sessionkeys.LanguageSessionName}, store))

	var session sessions.Session
	router.GET("/logout", func(c *gin.Context) {
		session = sessions.DefaultMany(c, sessionkeys.UserSessionName)
		session.Set("foobar", "fizzbuzz")
		require.NoError(t, session.Save())
		require.NotNil(t, session.Get("foobar"))
		auth.Logout(c, session)
	})

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/logout", nil)
	require.NoError(t, err)
	router.ServeHTTP(recorder, req)

	assert.Nil(t, session.Get("foobar"))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "<h1>Your progress has been saved</h1>")
}

func newAuthMiddlewareRouter(t *testing.T, authObj *authenticate.Auth, withToken bool, sessionValid bool) (*gin.Engine, *httptest.ResponseRecorder) {
	t.Helper()

	router := gin.Default()
	router.SetFuncMap(template.FuncMap{"WrapWelsh": webserver.WrapWelsh})
	router.LoadHTMLGlob("../templates/*")
	store := cookie.NewStore([]byte("secret"))
	router.Use(sessions.SessionsMany([]string{sessionkeys.SessionName, sessionkeys.UserSessionName, sessionkeys.SessionValidationName, sessionkeys.LanguageSessionName}, store))

	router.Use(func(c *gin.Context) {
		if withToken {
			session := sessions.DefaultMany(c, sessionkeys.UserSessionName)
			session.Set(authenticate.JWT_TOKEN_KEY, "foobar")
			require.NoError(t, session.Save())

			validationSession := sessions.DefaultMany(c, sessionkeys.SessionValidationName)
			validationSession.Set(authenticate.SESSION_VALID_KEY, sessionValid)
			require.NoError(t, validationSession.Save())
		}
		c.Next()
	})

	router.Use(authObj.AuthenticatedWithUAC)
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, true)
	})

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	require.NoError(t, err)
	router.ServeHTTP(recorder, req)
	return router, recorder
}

func TestAuthenticatedWithUac(t *testing.T) {
	newAuth := func(mockJwtCrypto *authmocks.JWTCryptoInterface) *authenticate.Auth {
		languageManagerMock := &languagemocks.LanguageManagerInterface{}
		languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
		return &authenticate.Auth{
			JWTCrypto:       mockJwtCrypto,
			CSRFManager:     &csrf.DefaultCSRFManager{Secret: "fwibble", SessionName: sessionkeys.SessionName},
			LanguageManager: languageManagerMock,
		}
	}

	t.Run("token decrypts and session valid", func(t *testing.T) {
		mockJwtCrypto := &authmocks.JWTCryptoInterface{}
		mockJwtCrypto.On("DecryptJWT", mock.Anything).Return(nil, nil)
		authObj := newAuth(mockJwtCrypto)
		_, recorder := newAuthMiddlewareRouter(t, authObj, true, true)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "true", recorder.Body.String())
	})

	t.Run("token decrypts but session invalid", func(t *testing.T) {
		mockJwtCrypto := &authmocks.JWTCryptoInterface{}
		mockJwtCrypto.On("DecryptJWT", mock.Anything).Return(nil, nil)
		authObj := newAuth(mockJwtCrypto)
		_, recorder := newAuthMiddlewareRouter(t, authObj, true, false)

		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Access study")
	})

	t.Run("token cannot be decrypted", func(t *testing.T) {
		mockJwtCrypto := &authmocks.JWTCryptoInterface{}
		mockJwtCrypto.On("DecryptJWT", mock.Anything).Return(nil, fmt.Errorf("Explosions"))
		authObj := newAuth(mockJwtCrypto)
		_, recorder := newAuthMiddlewareRouter(t, authObj, true, true)

		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Access study")
	})

	t.Run("no token", func(t *testing.T) {
		mockJwtCrypto := &authmocks.JWTCryptoInterface{}
		authObj := newAuth(mockJwtCrypto)
		_, recorder := newAuthMiddlewareRouter(t, authObj, false, false)

		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Access study")
	})
}

func TestHasSession(t *testing.T) {
	runHasSession := func(t *testing.T, claim *authenticate.UACClaims, decryptErr error) *httptest.ResponseRecorder {
		t.Helper()

		mockJwtCrypto := &authmocks.JWTCryptoInterface{}
		mockJwtCrypto.On("DecryptJWT", mock.Anything).Return(claim, decryptErr)
		languageManagerMock := &languagemocks.LanguageManagerInterface{}
		languageManagerMock.On("IsWelsh", mock.Anything).Return(false)

		authObj := &authenticate.Auth{
			JWTCrypto:       mockJwtCrypto,
			LanguageManager: languageManagerMock,
		}

		router := gin.Default()
		router.SetFuncMap(template.FuncMap{"WrapWelsh": webserver.WrapWelsh})
		router.LoadHTMLGlob("../templates/*")
		store := cookie.NewStore([]byte("secret"))
		router.Use(sessions.SessionsMany([]string{sessionkeys.SessionName, sessionkeys.UserSessionName, sessionkeys.SessionValidationName, sessionkeys.LanguageSessionName}, store))

		router.Use(func(c *gin.Context) {
			session := sessions.DefaultMany(c, sessionkeys.UserSessionName)
			session.Set(authenticate.JWT_TOKEN_KEY, "foobar")
			require.NoError(t, session.Save())
			c.Next()
		})

		router.GET("/", func(c *gin.Context) {
			hasSession, claimResult := authObj.HasSession(c)
			c.JSON(200, struct {
				HasSession bool
				Claim      *authenticate.UACClaims
			}{
				HasSession: hasSession,
				Claim:      claimResult,
			})
		})

		recorder := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "/", nil)
		require.NoError(t, err)
		router.ServeHTTP(recorder, req)
		return recorder
	}

	t.Run("returns true and claim", func(t *testing.T) {
		recorder := runHasSession(t, &authenticate.UACClaims{UACInfo: busapi.UACInfo{InstrumentName: "foobar", CaseID: "fizzbuzz", Disabled: false}}, nil)
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, `{"HasSession":true,"Claim":{"uac":"","auth_timeout":0,"instrument_name":"foobar","case_id":"fizzbuzz","disabled":false}}`, recorder.Body.String())
	})

	t.Run("returns disabled true", func(t *testing.T) {
		recorder := runHasSession(t, &authenticate.UACClaims{UACInfo: busapi.UACInfo{InstrumentName: "foobar", CaseID: "fizzbuzz", Disabled: true}}, nil)
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, `{"HasSession":true,"Claim":{"uac":"","auth_timeout":0,"instrument_name":"foobar","case_id":"fizzbuzz","disabled":true}}`, recorder.Body.String())
	})

	t.Run("returns false and empty claim when decrypt fails", func(t *testing.T) {
		recorder := runHasSession(t, nil, fmt.Errorf("Explosions"))
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, `{"HasSession":false,"Claim":null}`, recorder.Body.String())
	})
}

func TestForbidden(t *testing.T) {
	router := gin.Default()
	router.SetFuncMap(template.FuncMap{"WrapWelsh": webserver.WrapWelsh})
	router.LoadHTMLGlob("../templates/*")
	router.GET("/forbidden", func(c *gin.Context) {
		authenticate.Forbidden(c, true)
	})

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/forbidden", nil)
	require.NoError(t, err)
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "<html lang=\"cy\">")
	assert.Contains(t, recorder.Body.String(), "Mae'n ddrwg gennym, mae problem gyda'r gwasanaeth")
}

func TestRefreshToken(t *testing.T) {
	runRefresh := func(t *testing.T, initialToken interface{}, sessionValidValue interface{}, setupMock func(*authmocks.JWTCryptoInterface, *authenticate.UACClaims)) (sessions.Session, *httptest.ResponseRecorder, *authmocks.JWTCryptoInterface) {
		t.Helper()

		mockJwtCrypto := &authmocks.JWTCryptoInterface{}
		if setupMock != nil {
			claim := &authenticate.UACClaims{
				UAC:         "123456789012",
				AuthTimeout: 15,
				UACInfo: busapi.UACInfo{
					InstrumentName: "foo",
					CaseID:         "bar",
				},
			}
			setupMock(mockJwtCrypto, claim)
		}

		languageManagerMock := &languagemocks.LanguageManagerInterface{}
		languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
		authObj := &authenticate.Auth{
			JWTCrypto:       mockJwtCrypto,
			Logger:          zap.NewNop(),
			LanguageManager: languageManagerMock,
		}
		claim := &authenticate.UACClaims{
			UAC:         "123456789012",
			AuthTimeout: 15,
			UACInfo: busapi.UACInfo{
				InstrumentName: "foo",
				CaseID:         "bar",
			},
		}

		router := gin.Default()
		store := cookie.NewStore([]byte("secret"))
		router.Use(sessions.SessionsMany([]string{sessionkeys.UserSessionName, sessionkeys.SessionValidationName}, store))

		var userSession sessions.Session
		router.GET("/refresh", func(c *gin.Context) {
			userSession = sessions.DefaultMany(c, sessionkeys.UserSessionName)
			if initialToken != nil {
				userSession.Set(authenticate.JWT_TOKEN_KEY, initialToken)
				require.NoError(t, userSession.Save())
			}

			validationSession := sessions.DefaultMany(c, sessionkeys.SessionValidationName)
			validationSession.Set(authenticate.SESSION_VALID_KEY, sessionValidValue)
			require.NoError(t, validationSession.Save())

			authObj.RefreshToken(c, userSession, claim)
			c.Status(http.StatusNoContent)
		})

		recorder := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "/refresh", nil)
		require.NoError(t, err)
		router.ServeHTTP(recorder, req)

		return userSession, recorder, mockJwtCrypto
	}

	t.Run("refreshes token when existing and session valid", func(t *testing.T) {
		session, recorder, _ := runRefresh(t, "existing-token", true, func(m *authmocks.JWTCryptoInterface, claim *authenticate.UACClaims) {
			m.On("EncryptJWT", claim.UAC, &claim.UACInfo, claim.AuthTimeout).Return("refreshed-token", nil).Once()
		})
		assert.Equal(t, http.StatusNoContent, recorder.Code)
		assert.Equal(t, "refreshed-token", session.Get(authenticate.JWT_TOKEN_KEY))
	})

	t.Run("does not refresh when no existing token", func(t *testing.T) {
		_, recorder, mockJwtCrypto := runRefresh(t, nil, true, nil)
		assert.Equal(t, http.StatusNoContent, recorder.Code)
		assert.Empty(t, mockJwtCrypto.Calls)
	})

	t.Run("does not refresh when session invalid", func(t *testing.T) {
		session, recorder, mockJwtCrypto := runRefresh(t, "existing-token", false, nil)
		assert.Equal(t, http.StatusNoContent, recorder.Code)
		assert.Equal(t, "existing-token", session.Get(authenticate.JWT_TOKEN_KEY))
		assert.Empty(t, mockJwtCrypto.Calls)
	})

	t.Run("keeps existing token when encryption fails", func(t *testing.T) {
		session, recorder, _ := runRefresh(t, "existing-token", true, func(m *authmocks.JWTCryptoInterface, claim *authenticate.UACClaims) {
			m.On("EncryptJWT", claim.UAC, &claim.UACInfo, claim.AuthTimeout).Return("", fmt.Errorf("encrypt failed")).Once()
		})
		assert.Equal(t, http.StatusNoContent, recorder.Code)
		assert.Equal(t, "existing-token", session.Get(authenticate.JWT_TOKEN_KEY))
	})

	t.Run("does not refresh when token has unexpected type", func(t *testing.T) {
		session, recorder, mockJwtCrypto := runRefresh(t, 123, true, nil)
		assert.Equal(t, http.StatusNoContent, recorder.Code)
		assert.Equal(t, 123, session.Get(authenticate.JWT_TOKEN_KEY))
		assert.Empty(t, mockJwtCrypto.Calls)
	})

	t.Run("does not refresh when session validation type is unexpected", func(t *testing.T) {
		session, recorder, mockJwtCrypto := runRefresh(t, "existing-token", "true", nil)
		assert.Equal(t, http.StatusNoContent, recorder.Code)
		assert.Equal(t, "existing-token", session.Get(authenticate.JWT_TOKEN_KEY))
		assert.Empty(t, mockJwtCrypto.Calls)
	})
}

func TestLoginWelshValidation(t *testing.T) {
	runWelshLogin := func(t *testing.T, uacKind string) *httptest.ResponseRecorder {
		t.Helper()

		languageManagerMock := &languagemocks.LanguageManagerInterface{}
		languageManagerMock.On("IsWelsh", mock.Anything).Return(true)
		authObj := &authenticate.Auth{
			Logger:          zap.NewNop(),
			CSRFManager:     &csrf.DefaultCSRFManager{Secret: "fwibble", SessionName: sessionkeys.SessionName},
			LanguageManager: languageManagerMock,
			UACKind:         uacKind,
		}

		router := gin.Default()
		router.SetFuncMap(template.FuncMap{"WrapWelsh": webserver.WrapWelsh})
		router.LoadHTMLGlob("../templates/*")
		store := cookie.NewStore([]byte("secret"))
		router.Use(sessions.SessionsMany([]string{sessionkeys.SessionName, sessionkeys.UserSessionName, sessionkeys.SessionValidationName, sessionkeys.LanguageSessionName}, store))
		router.POST("/login", func(c *gin.Context) {
			session := sessions.DefaultMany(c, sessionkeys.UserSessionName)
			authObj.Login(c, session)
		})

		recorder := httptest.NewRecorder()
		data := url.Values{"uac": []string{""}}
		req, err := http.NewRequest(http.MethodPost, "/login", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		router.ServeHTTP(recorder, req)

		return recorder
	}

	t.Run("12-digit mode", func(t *testing.T) {
		recorder := runWelshLogin(t, "uac")
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Rhowch eich cod mynediad sy'n cynnwys 12 o nodau")
	})

	t.Run("16-character mode", func(t *testing.T) {
		recorder := runWelshLogin(t, "uac16")
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Rhowch eich cod mynediad sy'n cynnwys 16 o nodau")
	})
}
