package authenticate_test

import (
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	mockauth "github.com/ONSdigital/blaise-cawi-portal/authenticate/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/blaiserestapi"
	mockrestapi "github.com/ONSdigital/blaise-cawi-portal/blaiserestapi/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/ONSdigital/blaise-cawi-portal/busapi/mocks"
	languageManagerMocks "github.com/ONSdigital/blaise-cawi-portal/languagemanager/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/webserver"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	csrf "github.com/srbry/gin-csrf"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Login", func() {
	var (
		shortUAC    = "22222"
		longUAC     = "11112222333344445555"
		spacedUAC   = "1234 5678 9012"
		spacedUAC16 = "bcdf 5678 ghjk 2345"
		validUAC    = "123456789012"
		validUAC16  = "bcdf5678ghjk2345"
		jwtCrypto   = &authenticate.JWTCrypto{
			JWTSecret: "hello",
		}
		languageManagerMock *languageManagerMocks.LanguageManagerInterface
		auth                *authenticate.Auth
		httpRouter          *gin.Engine
		httpRecorder        *httptest.ResponseRecorder
		session             sessions.Session
		observedLogs        *observer.ObservedLogs
		observedZapCore     zapcore.Core
		csrfManager         = &csrf.DefaultCSRFManager{
			Secret:      "fwibble",
			SessionName: "session",
		}
	)

	BeforeEach(func() {
		observedZapCore, observedLogs = observer.New(zap.InfoLevel)
		observedLogger := zap.New(observedZapCore)
		languageManagerMock = &languageManagerMocks.LanguageManagerInterface{}
		languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
		languageManagerMock.On("LanguageError", mock.Anything, mock.Anything).Return("Access code not recognised. Enter the code again")
		auth = &authenticate.Auth{
			JWTCrypto:       jwtCrypto,
			Logger:          observedLogger,
			CSRFManager:     csrfManager,
			LanguageManager: languageManagerMock,
		}
		httpRouter = gin.Default()
		httpRouter.SetFuncMap(template.FuncMap{
			"WrapWelsh": webserver.WrapWelsh,
		})
		httpRouter.LoadHTMLGlob("../templates/*")
		store := cookie.NewStore([]byte("secret"))
		httpRouter.Use(sessions.SessionsMany([]string{"session", "user_session", "session_validation", "language_session"}, store))
		httpRouter.POST("/login", func(context *gin.Context) {
			session = sessions.DefaultMany(context, "user_session")
			auth.Login(context, session)
		})
	})

	Context("When an instrument is not installed", func() {
		var uacValue string

		JustBeforeEach(func() {
			httpRecorder = httptest.NewRecorder()
			data := url.Values{
				"uac": []string{uacValue},
			}
			req, _ := http.NewRequest("POST", "/login", strings.NewReader(data.Encode()))
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			httpRouter.ServeHTTP(httpRecorder, req)
		})

		BeforeEach(func() {
			uacValue = validUAC
			auth.UacKind = "uac"
			mockBusApi := &mocks.BusApiInterface{}
			auth.BusApi = mockBusApi

			mockBusApi.On("GetUacInfo", validUAC).Once().Return(busapi.UacInfo{InstrumentName: "foo", CaseID: "bar"}, nil)

			mockRestApi := &mockrestapi.BlaiseRestApiInterface{}
			auth.BlaiseRestApi = mockRestApi
			mockRestApi.On("GetInstrumentSettings", mock.Anything).Return(blaiserestapi.InstrumentSettings{}, blaiserestapi.InstrumentNotFoundError)
		})

		It("returns the not live page", func() {
			Expect(httpRecorder.Code).To(Equal(http.StatusOK))
			body := httpRecorder.Body.Bytes()
			Expect(strings.Contains(string(body), `The study is currently unavailable`)).To(BeTrue())
			Expect(strings.Contains(string(body), `Please try again later or contact our Survey Enquiry Line on 0800 085 7376 for help.`)).To(BeTrue())
			Expect(strings.Contains(string(body), `Any answers you have provided in previous sessions have been logged securely and confidentially. They will only be used for the purposes of this research.`)).To(BeTrue())
		})

		It("logs a warning", func() {
			Expect(observedLogs.Len()).To(Equal(1))
			Expect(observedLogs.All()[0].Message).To(Equal("Failed Login"))
			Expect(observedLogs.All()[0].ContextMap()["Reason"]).To(Equal("Instrument not installed"))
			Expect(observedLogs.All()[0].ContextMap()["Notes"]).To(Equal("This can happen if a UAC for a non-Blaise 5 survey has been entered"))
			Expect(observedLogs.All()[0].ContextMap()["InstrumentName"]).To(Equal("foo"))
			Expect(observedLogs.All()[0].ContextMap()["CaseID"]).To(Equal("bar"))
			Expect(observedLogs.All()[0].Level).To(Equal(zap.WarnLevel))
		})
	})

	Context("When instrument settings does not error", func() {
		BeforeEach(func() {
			mockRestApi := &mockrestapi.BlaiseRestApiInterface{}
			auth.BlaiseRestApi = mockRestApi
			mockRestApi.On("GetInstrumentSettings", mock.Anything).Return(blaiserestapi.InstrumentSettings{}, nil)
		})

		Context("Login with a correct length, invalid UAC Code", func() {
			var uacValue string

			JustBeforeEach(func() {
				httpRecorder = httptest.NewRecorder()
				data := url.Values{
					"uac": []string{uacValue},
				}
				req, _ := http.NewRequest("POST", "/login", strings.NewReader(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				req.RemoteAddr = "1.1.1.1"
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			BeforeEach(func() {
				uacValue = validUAC
				auth.UacKind = "uac"
				mockBusApi := &mocks.BusApiInterface{}
				auth.BusApi = mockBusApi

				mockBusApi.On("GetUacInfo", validUAC).Once().Return(busapi.UacInfo{InstrumentName: "", CaseID: "bar"}, nil)
			})

			It("returns a status unauthorised with an error", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
				Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
				Expect(session.Get(authenticate.JWT_TOKEN_KEY)).To(BeNil())
				body := httpRecorder.Body.Bytes()
				Expect(string(body)).To(ContainSubstring(`Access code not recognised. Enter the code again`))

				Expect(observedLogs.Len()).To(Equal(1))
				Expect(observedLogs.All()[0].Message).To(Equal("Failed Login"))
				Expect(observedLogs.All()[0].ContextMap()["SourceIP"]).To(Equal("1.1.1.1"))
				Expect(observedLogs.All()[0].ContextMap()["Reason"]).To(Equal("Access code not recognised"))
				Expect(observedLogs.All()[0].ContextMap()["InstrumentName"]).To(Equal(""))
				Expect(observedLogs.All()[0].ContextMap()["CaseID"]).To(Equal("bar"))
				Expect(observedLogs.All()[0].ContextMap()["error"]).To(BeNil())
				Expect(observedLogs.All()[0].Level).To(Equal(zap.InfoLevel))
			})
		})

		Context("Login with a valid UAC Code", func() {
			var uacValue string

			JustBeforeEach(func() {
				httpRecorder = httptest.NewRecorder()
				data := url.Values{
					"uac": []string{uacValue},
				}
				req, _ := http.NewRequest("POST", "/login", strings.NewReader(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			Context("Login with a 12 digit UAC kind", func() {
				BeforeEach(func() {
					uacValue = validUAC
					auth.UacKind = "uac"
					mockBusApi := &mocks.BusApiInterface{}
					auth.BusApi = mockBusApi

					mockBusApi.On("GetUacInfo", validUAC).Once().Return(busapi.UacInfo{InstrumentName: "foo", CaseID: "bar"}, nil)
				})

				It("redirects to /:instrumentName/", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusFound))
					Expect(httpRecorder.Header()["Location"]).To(Equal([]string{"/foo/"}))
					Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
					decryptedToken, _ := auth.JWTCrypto.DecryptJWT(session.Get(authenticate.JWT_TOKEN_KEY))
					Expect(decryptedToken.UAC).To(Equal(validUAC))
					Expect(decryptedToken.UacInfo.InstrumentName).To(Equal("foo"))
					Expect(decryptedToken.UacInfo.CaseID).To(Equal("bar"))
					Expect(session.Get(authenticate.SESSION_TIMEOUT_KEY).(int)).To(Equal(15))



				})
			})

			Context("Login with a 16 character UAC kind", func() {
				BeforeEach(func() {
					uacValue = validUAC16
					auth.UacKind = "uac16"
					mockBusApi := &mocks.BusApiInterface{}
					auth.BusApi = mockBusApi

					mockBusApi.On("GetUacInfo", validUAC16).Once().Return(busapi.UacInfo{InstrumentName: "foo", CaseID: "bar"}, nil)
				})

				It("redirects to /:instrumentName/", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusFound))
					Expect(httpRecorder.Header()["Location"]).To(Equal([]string{"/foo/"}))
					Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
					decryptedToken, _ := auth.JWTCrypto.DecryptJWT(session.Get(authenticate.JWT_TOKEN_KEY))
					Expect(decryptedToken.UAC).To(Equal(validUAC16))
					Expect(decryptedToken.UacInfo.InstrumentName).To(Equal("foo"))
					Expect(decryptedToken.UacInfo.CaseID).To(Equal("bar"))
					Expect(session.Get(authenticate.SESSION_TIMEOUT_KEY).(int)).To(Equal(15))
				})
			})

		})

		Context("Login with a valid UAC Code containing whitespace", func() {
			var uacValue string

			JustBeforeEach(func() {
				httpRecorder = httptest.NewRecorder()
				data := url.Values{
					"uac": []string{uacValue},
				}
				req, _ := http.NewRequest("POST", "/login", strings.NewReader(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			Context("Login with a 12 digit UAC kind", func() {
				BeforeEach(func() {
					uacValue = spacedUAC
					auth.UacKind = "uac"
					mockBusApi := &mocks.BusApiInterface{}
					auth.BusApi = mockBusApi

					mockBusApi.On("GetUacInfo", validUAC).Once().Return(busapi.UacInfo{InstrumentName: "foo", CaseID: "bar"}, nil)
				})

				It("redirects to /:instrumentName/", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusFound))
					Expect(httpRecorder.Header()["Location"]).To(Equal([]string{"/foo/"}))
					Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
					decryptedToken, _ := auth.JWTCrypto.DecryptJWT(session.Get(authenticate.JWT_TOKEN_KEY))
					Expect(decryptedToken.UAC).To(Equal(validUAC))
					Expect(decryptedToken.UacInfo.InstrumentName).To(Equal("foo"))
					Expect(decryptedToken.UacInfo.CaseID).To(Equal("bar"))

					Expect(observedLogs.Len()).To(Equal(1))
					Expect(observedLogs.All()[0].Message).To(ContainSubstring("Successful Login with InstrumentName: foo"))
				})
			})

			Context("Login with a 16 character UAC kind", func() {
				BeforeEach(func() {
					uacValue = spacedUAC16
					auth.UacKind = "uac16"
					mockBusApi := &mocks.BusApiInterface{}
					auth.BusApi = mockBusApi

					mockBusApi.On("GetUacInfo", validUAC16).Once().Return(busapi.UacInfo{InstrumentName: "foo", CaseID: "bar"}, nil)
				})

				It("redirects to /:instrumentName/", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusFound))
					Expect(httpRecorder.Header()["Location"]).To(Equal([]string{"/foo/"}))
					Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
					decryptedToken, _ := auth.JWTCrypto.DecryptJWT(session.Get(authenticate.JWT_TOKEN_KEY))
					Expect(decryptedToken.UAC).To(Equal(validUAC16))
					Expect(decryptedToken.UacInfo.InstrumentName).To(Equal("foo"))
					Expect(decryptedToken.UacInfo.CaseID).To(Equal("bar"))
				})
			})
		})

		Context("Login with a short UAC Code", func() {
			JustBeforeEach(func() {
				httpRecorder = httptest.NewRecorder()
				data := url.Values{
					"uac": []string{shortUAC},
				}
				req, _ := http.NewRequest("POST", "/login", strings.NewReader(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			Context("Login with a 12 digit UAC kind", func() {
				BeforeEach(func() {
					auth.UacKind = "uac"
				})

				It("returns a status unauthorised with an error", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
					Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
					Expect(session.Get(authenticate.JWT_TOKEN_KEY)).To(BeNil())
					body := httpRecorder.Body.Bytes()
					Expect(strings.Contains(string(body), `Enter your 12-digit access code`)).To(BeTrue())
				})
			})

			Context("Login with a 16 character UAC kind", func() {
				BeforeEach(func() {
					auth.UacKind = "uac16"
				})

				It("returns a status unauthorised with an error", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
					Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
					Expect(session.Get(authenticate.JWT_TOKEN_KEY)).To(BeNil())
					body := httpRecorder.Body.Bytes()
					Expect(strings.Contains(string(body), `Enter your 16-character access code`)).To(BeTrue())
				})
			})
		})

		Context("Login with a long UAC Code", func() {
			JustBeforeEach(func() {
				httpRecorder = httptest.NewRecorder()
				data := url.Values{
					"uac": []string{longUAC},
				}
				req, _ := http.NewRequest("POST", "/login", strings.NewReader(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				req.RemoteAddr = "1.1.1.1"
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			Context("Login with a 12 digit UAC kind", func() {
				BeforeEach(func() {
					auth.UacKind = "uac"
				})

				It("returns a status unauthorised with an error", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
					Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
					Expect(session.Get(authenticate.JWT_TOKEN_KEY)).To(BeNil())
					body := httpRecorder.Body.Bytes()
					Expect(strings.Contains(string(body), `Enter your 12-digit access code`)).To(BeTrue())

					Expect(observedLogs.Len()).To(Equal(1))
					Expect(observedLogs.All()[0].Message).To(Equal("Failed Login"))
					Expect(observedLogs.All()[0].ContextMap()["SourceIP"]).To(Equal("1.1.1.1"))
					Expect(observedLogs.All()[0].ContextMap()["Reason"]).To(Equal("Invalid UAC length"))
					Expect(observedLogs.All()[0].ContextMap()["UACLength"]).To(Equal(int64(12)))
					Expect(observedLogs.All()[0].Level).To(Equal(zap.InfoLevel))
				})
			})

			Context("Login with a 16 character UAC kind", func() {
				BeforeEach(func() {
					auth.UacKind = "uac16"
				})

				It("returns a status unauthorised with an error", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
					Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
					Expect(session.Get(authenticate.JWT_TOKEN_KEY)).To(BeNil())
					body := httpRecorder.Body.Bytes()
					Expect(strings.Contains(string(body), `Enter your 16-character access code`)).To(BeTrue())

					Expect(observedLogs.Len()).To(Equal(1))
					Expect(observedLogs.All()[0].Message).To(Equal("Failed Login"))
					Expect(observedLogs.All()[0].ContextMap()["SourceIP"]).To(Equal("1.1.1.1"))
					Expect(observedLogs.All()[0].ContextMap()["Reason"]).To(Equal("Invalid UAC length"))
					Expect(observedLogs.All()[0].ContextMap()["UACLength"]).To(Equal(int64(16)))
					Expect(observedLogs.All()[0].Level).To(Equal(zap.InfoLevel))
				})
			})
		})

		Context("Login with no UAC Code", func() {
			JustBeforeEach(func() {
				httpRecorder = httptest.NewRecorder()
				data := url.Values{
					"uac": []string{},
				}
				req, _ := http.NewRequest("POST", "/login", strings.NewReader(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				req.RemoteAddr = "1.1.1.1"
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			Context("Login with a 12 digit UAC kind", func() {
				BeforeEach(func() {
					auth.UacKind = "uac"
				})

				It("returns a status unauthorised with an error", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
					Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
					Expect(session.Get(authenticate.JWT_TOKEN_KEY)).To(BeNil())
					body := httpRecorder.Body.Bytes()
					Expect(strings.Contains(string(body), `Enter your 12-digit access code`)).To(BeTrue())

					Expect(observedLogs.Len()).To(Equal(1))
					Expect(observedLogs.All()[0].Message).To(Equal("Failed Login"))
					Expect(observedLogs.All()[0].ContextMap()["SourceIP"]).To(Equal("1.1.1.1"))
					Expect(observedLogs.All()[0].ContextMap()["Reason"]).To(Equal("Blank UAC"))
					Expect(observedLogs.All()[0].Level).To(Equal(zap.InfoLevel))
				})
			})

			Context("Login with a 16 character UAC kind", func() {
				BeforeEach(func() {
					auth.UacKind = "uac16"
				})

				It("returns a status unauthorised with an error", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
					Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
					Expect(session.Get(authenticate.JWT_TOKEN_KEY)).To(BeNil())
					body := httpRecorder.Body.Bytes()
					Expect(strings.Contains(string(body), `Enter your 16-character access code`)).To(BeTrue())
					Expect(observedLogs.Len()).To(Equal(1))

					Expect(observedLogs.All()[0].Message).To(Equal("Failed Login"))
					Expect(observedLogs.All()[0].ContextMap()["SourceIP"]).To(Equal("1.1.1.1"))
					Expect(observedLogs.All()[0].ContextMap()["Reason"]).To(Equal("Blank UAC"))
					Expect(observedLogs.All()[0].Level).To(Equal(zap.InfoLevel))
				})
			})
		})
	})

	var _ = Describe("Logout", func() {
		var (
			httpRouter   *gin.Engine
			httpRecorder *httptest.ResponseRecorder
			session      sessions.Session
			csrfManager  = &csrf.DefaultCSRFManager{
				Secret:      "fwibble",
				SessionName: "session",
			}
			languageManagerMock = &languageManagerMocks.LanguageManagerInterface{}
			auth                = &authenticate.Auth{CSRFManager: csrfManager, LanguageManager: languageManagerMock}
		)

		BeforeEach(func() {
			languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
			httpRouter = gin.Default()
			httpRouter.SetFuncMap(template.FuncMap{
				"WrapWelsh": webserver.WrapWelsh,
			})
			httpRouter.LoadHTMLGlob("../templates/*")
			store := cookie.NewStore([]byte("secret"))
			httpRouter.Use(sessions.SessionsMany([]string{"session", "user_session", "session_validation"}, store))
			httpRouter.GET("/logout", func(context *gin.Context) {
				session = sessions.DefaultMany(context, "user_session")
				session.Set("foobar", "fizzbuzz")
				_ = session.Save()
				Expect(session.Get("foobar")).ToNot(BeNil())
				auth.Logout(context, session)
			})
		})

		Context("Logout of a session", func() {
			JustBeforeEach(func() {
				httpRecorder = httptest.NewRecorder()
				req, _ := http.NewRequest("GET", "/logout", nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			It("Clears the current session and renders the log out confirmation page", func() {
				Expect(session.Get("foobar")).To(BeNil())
				Expect(httpRecorder.Code).To(Equal(http.StatusOK))
				body := httpRecorder.Body.Bytes()
				Expect(strings.Contains(string(body), `<h1>Your progress has been saved</h1>`)).To(BeTrue())
			})
		})
	})
})

var _ = Describe("AuthenticatedWithUac", func() {
	var (
		session sessions.Session

		mockJwtCrypto = &mockauth.JWTCryptoInterface{}
		csrfManager   = &csrf.DefaultCSRFManager{
			Secret:      "fwibble",
			SessionName: "session",
		}
		languageManagerMock = &languageManagerMocks.LanguageManagerInterface{}
		auth                = &authenticate.Auth{
			JWTCrypto:       mockJwtCrypto,
			CSRFManager:     csrfManager,
			LanguageManager: languageManagerMock,
		}
		httpRecorder *httptest.ResponseRecorder
		httpRouter   *gin.Engine
		sessionValid = false
	)

	BeforeEach(func() {
		languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
		httpRouter = gin.Default()
		httpRouter.SetFuncMap(template.FuncMap{
			"WrapWelsh": webserver.WrapWelsh,
		})
		httpRouter.LoadHTMLGlob("../templates/*")
		store := cookie.NewStore([]byte("secret"))
		httpRouter.Use(sessions.SessionsMany([]string{"session", "user_session", "session_validation", "language_session"}, store))
	})

	AfterEach(func() {
		httpRouter = gin.Default()
		mockJwtCrypto = &mockauth.JWTCryptoInterface{}
		auth.JWTCrypto = mockJwtCrypto
		languageManagerMock = &languageManagerMocks.LanguageManagerInterface{}
		auth.LanguageManager = languageManagerMock
	})

	JustBeforeEach(func() {
		httpRecorder = httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		httpRouter.ServeHTTP(httpRecorder, req)
	})

	Context("when there is a token", func() {
		BeforeEach(func() {
			httpRouter.Use(func(context *gin.Context) {
				session = sessions.DefaultMany(context, "user_session")
				session.Set(authenticate.JWT_TOKEN_KEY, "foobar")
				_ = session.Save()

				sessionValidation := sessions.DefaultMany(context, "session_validation")
				sessionValidation.Set(authenticate.SESSION_VALID_KEY, sessionValid)
				_ = sessionValidation.Save()
				context.Next()
			})

			httpRouter.Use(auth.AuthenticatedWithUac)
			httpRouter.GET("/", func(context *gin.Context) {
				context.JSON(200, true)
			})
		})

		Context("When a token can be decrypted", func() {
			BeforeEach(func() {
				mockJwtCrypto.On("DecryptJWT", mock.Anything).Return(nil, nil)
			})

			Context("and the session is valid", func() {
				BeforeEach(func() {
					sessionValid = true
				})

				It("Allows the context to continue", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusOK))
					body := httpRecorder.Body.Bytes()
					Expect(string(body)).To(Equal("true"))
				})
			})

			Context("and the session is invalid", func() {
				BeforeEach(func() {
					sessionValid = false
				})

				It("returns unauthorized", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
					body := httpRecorder.Body.Bytes()
					Expect(string(body)).To(ContainSubstring(`Access study`))
				})
			})
		})

		Context("When a token cannot be decrypted", func() {
			BeforeEach(func() {
				mockJwtCrypto.On("DecryptJWT", mock.Anything).Return(nil, fmt.Errorf("Explosions"))
			})

			It("returns unauthorized", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
				body := httpRecorder.Body.Bytes()
				Expect(string(body)).To(ContainSubstring(`Access study`))
			})
		})
	})

	Context("When there is no token", func() {
		BeforeEach(func() {
			httpRouter.Use(auth.AuthenticatedWithUac)
			httpRouter.GET("/", func(context *gin.Context) {
				context.JSON(200, true)
			})
		})

		It("returns unauthorized", func() {
			Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
			body := httpRecorder.Body.Bytes()
			Expect(string(body)).To(ContainSubstring(`Access study`))
		})
	})
})

var _ = Describe("Has Session", func() {
	var (
		session sessions.Session

		mockJwtCrypto       = &mockauth.JWTCryptoInterface{}
		languageManagerMock = &languageManagerMocks.LanguageManagerInterface{}
		auth                = &authenticate.Auth{
			JWTCrypto:       mockJwtCrypto,
			LanguageManager: languageManagerMock,
		}
		httpRecorder   *httptest.ResponseRecorder
		httpRouter     *gin.Engine
		instrumentName = "foobar"
		caseID         = "fizzbuzz"
	)

	BeforeEach(func() {
		languageManagerMock = &languageManagerMocks.LanguageManagerInterface{}
		languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
		httpRouter = gin.Default()
		httpRouter.SetFuncMap(template.FuncMap{
			"WrapWelsh": webserver.WrapWelsh,
		})
		httpRouter.LoadHTMLGlob("../templates/*")
		store := cookie.NewStore([]byte("secret"))
		httpRouter.Use(sessions.SessionsMany([]string{"session", "user_session", "session_validation", "language_session"}, store))

		httpRouter.Use(func(context *gin.Context) {
			session = sessions.DefaultMany(context, "user_session")
			session.Set(authenticate.JWT_TOKEN_KEY, "foobar")
			_ = session.Save()
			context.Next()
		})

		httpRouter.GET("/", func(context *gin.Context) {
			hasSession, claim := auth.HasSession(context)
			context.JSON(200, struct {
				HasSession bool
				Claim      *authenticate.UACClaims
			}{
				HasSession: hasSession,
				Claim:      claim,
			})
		})
	})

	AfterEach(func() {
		mockJwtCrypto = &mockauth.JWTCryptoInterface{}
		auth.JWTCrypto = mockJwtCrypto
	})

	JustBeforeEach(func() {
		httpRecorder = httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		httpRouter.ServeHTTP(httpRecorder, req)
	})

	Context("When someone has a session", func() {
		BeforeEach(func() {
			mockJwtCrypto.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{
				UacInfo: busapi.UacInfo{
					InstrumentName: instrumentName,
					CaseID:         caseID,
				},
			}, nil)
		})

		It("returns true and a claim", func() {
			Expect(httpRecorder.Code).To(Equal(http.StatusOK))
			body := httpRecorder.Body.Bytes()
			Expect(string(body)).To(Equal(
				`{"HasSession":true,"Claim":{"uac":"","auth_timeout":0,"instrument_name":"foobar","case_id":"fizzbuzz"}}`,
			))
		})
	})

	Context("When someone doesn't have a session", func() {
		BeforeEach(func() {
			mockJwtCrypto.On("DecryptJWT", mock.Anything).Return(nil, fmt.Errorf("Explosions"))
		})

		It("returns false and an empty claim", func() {
			Expect(httpRecorder.Code).To(Equal(http.StatusOK))
			body := httpRecorder.Body.Bytes()
			Expect(string(body)).To(Equal(`{"HasSession":false,"Claim":null}`))
		})
	})
})
