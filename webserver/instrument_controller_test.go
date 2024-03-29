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

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/authenticate/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	languageManagerMocks "github.com/ONSdigital/blaise-cawi-portal/languagemanager/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/webserver"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// A response recorder that supports streams (for efficent proxying)
type TestResponseRecorder struct {
	*httptest.ResponseRecorder
	closeChannel chan bool
}

func (r *TestResponseRecorder) CloseNotify() <-chan bool {
	return r.closeChannel
}

func CreateTestResponseRecorder() *TestResponseRecorder {
	return &TestResponseRecorder{
		httptest.NewRecorder(),
		make(chan bool, 1),
	}
}

type ErrReader struct{ Error error }

func (e *ErrReader) Read([]byte) (int, error) {
	return 0, e.Error
}

var _ = Describe("Open Case", func() {
	var (
		catiUrl              = "http://localhost"
		instrumentName       = "foobar"
		caseID               = "fizzbuzz"
		httpRouter           *gin.Engine
		httpRecorder         *TestResponseRecorder
		responseInfo         = "<html><head></head><body></body></html>"
		mockAuth             = &mocks.AuthInterface{}
		mockJWTCrypto        = &mocks.JWTCryptoInterface{}
		languageManagerMock  = &languageManagerMocks.LanguageManagerInterface{}
		instrumentController = &webserver.InstrumentController{CatiUrl: catiUrl, HttpClient: &http.Client{}, Auth: mockAuth, JWTCrypto: mockJWTCrypto, LanguageManager: languageManagerMock}
		requestBody          io.Reader
		observedLogs         *observer.ObservedLogs
		observedZapCore      zapcore.Core
	)

	BeforeEach(func() {
		httpRouter = gin.Default()
		httpRouter.SetFuncMap(template.FuncMap{
			"WrapWelsh": webserver.WrapWelsh,
		})
		httpRouter.LoadHTMLGlob("../templates/*")
		store := cookie.NewStore([]byte("secret"))
		httpRouter.Use(sessions.SessionsMany([]string{"session", "user_session", "session_validation", "language_session"}, store))
		observedZapCore, observedLogs = observer.New(zap.InfoLevel)
		observedLogger := zap.New(observedZapCore)
		_ = observedLogger.Sync()
		instrumentController.Logger = observedLogger
		instrumentController.AddRoutes(httpRouter)
		httpmock.Activate()
		mockAuth.On("RefreshToken", mock.Anything, mock.Anything, mock.Anything).Return()
	})

	AfterEach(func() {
		httpmock.DeactivateAndReset()
		mockAuth = &mocks.AuthInterface{}
		languageManagerMock = &languageManagerMocks.LanguageManagerInterface{}
		instrumentController.Auth = mockAuth
		instrumentController.LanguageManager = languageManagerMock
		mockJWTCrypto = &mocks.JWTCryptoInterface{}
		instrumentController.JWTCrypto = mockJWTCrypto
	})

	Describe("Open a case in Blaise", func() {
		Context("Launching Blaise in Cawi mode with a valid instrument and case id", func() {
			Context("and the script can be injected", func() {
				JustBeforeEach(func() {
					languageManagerMock.On("IsWelsh", mock.Anything).Return(false)

					mockResponse := &http.Response{
						StatusCode: 200,
						Header: http.Header{
							"Content-Type": {"text/html"},
						},
						Body: io.NopCloser(strings.NewReader(responseInfo)),
					}
					httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/default.aspx", catiUrl, instrumentName),
						httpmock.ResponderFromResponse(mockResponse))

					mockAuth.On("AuthenticatedWithUac", mock.Anything).Return()
					mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{UacInfo: busapi.UacInfo{
						InstrumentName: instrumentName,
						CaseID:         caseID,
					}}, nil)
					instrumentController.Auth = mockAuth

					httpRecorder = CreateTestResponseRecorder()
					req, _ := http.NewRequest("GET", fmt.Sprintf("/%s/", instrumentName), nil)
					httpRouter.ServeHTTP(httpRecorder, req)
				})

				It("Returns a 200 response and some data, with an injected check-session script", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusOK))
					Expect(httpRecorder.Body.String()).To(Equal(`<html><head></head><body><script src="/assets/js/check-session.js"></script></body></html>`))
				})
			})
		})

		Context("Launching Blaise in Cawi mode for a different instrument", func() {
			Context("Welsh", func() {
				BeforeEach(func() {
					languageManagerMock.On("IsWelsh", mock.Anything).Return(true)
				})

				JustBeforeEach(func() {
					httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/default.aspx", catiUrl, instrumentName),
						httpmock.NewStringResponder(200, responseInfo))

					mockAuth.On("AuthenticatedWithUac", mock.Anything).Return()
					mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{UacInfo: busapi.UacInfo{
						InstrumentName: instrumentName,
						CaseID:         caseID,
					}}, nil)

					httpRecorder = CreateTestResponseRecorder()
					req, _ := http.NewRequest("GET", fmt.Sprintf("/%s/", "fwibble"), nil)
					httpRouter.ServeHTTP(httpRecorder, req)
				})

				It("Returns a 403", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusForbidden))
					Expect(httpRecorder.Body.String()).To(ContainSubstring(
						`I fynd i'r dudalen hon, bydd angen i chi .<a href="/">roi eich cod mynediad eto</a>.`,
					))

					Expect(observedLogs.Len()).To(Equal(1))
					Expect(observedLogs.All()[0].Message).To(Equal("Not authenticated for instrument"))
					Expect(observedLogs.All()[0].ContextMap()["AuthedCaseID"]).To(Equal(caseID))
					Expect(observedLogs.All()[0].ContextMap()["AuthedInstrumentName"]).To(Equal(instrumentName))
					Expect(observedLogs.All()[0].ContextMap()["InstrumentName"]).To(Equal("fwibble"))
					Expect(observedLogs.All()[0].Level).To(Equal(zap.InfoLevel))
				})
			})

			Context("English", func() {
				BeforeEach(func() {
					languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
				})

				JustBeforeEach(func() {
					httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/default.aspx", catiUrl, instrumentName),
						httpmock.NewStringResponder(200, responseInfo))

					mockAuth.On("AuthenticatedWithUac", mock.Anything).Return()
					mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{UacInfo: busapi.UacInfo{
						InstrumentName: instrumentName,
						CaseID:         caseID,
					}}, nil)

					httpRecorder = CreateTestResponseRecorder()
					req, _ := http.NewRequest("GET", fmt.Sprintf("/%s/", "fwibble"), nil)
					httpRouter.ServeHTTP(httpRecorder, req)
				})

				It("Returns a 403", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusForbidden))
					Expect(httpRecorder.Body.String()).To(ContainSubstring(
						`To access this page you need to <a href="/">re-enter your access code</a>`,
					))

					Expect(observedLogs.Len()).To(Equal(1))
					Expect(observedLogs.All()[0].Message).To(Equal("Not authenticated for instrument"))
					Expect(observedLogs.All()[0].ContextMap()["AuthedCaseID"]).To(Equal(caseID))
					Expect(observedLogs.All()[0].ContextMap()["AuthedInstrumentName"]).To(Equal(instrumentName))
					Expect(observedLogs.All()[0].ContextMap()["InstrumentName"]).To(Equal("fwibble"))
					Expect(observedLogs.All()[0].Level).To(Equal(zap.InfoLevel))
				})
			})
		})

		Context("When failing to decrupt a JWT", func() {
			JustBeforeEach(func() {
				languageManagerMock.On("LanguageError", mock.Anything, mock.Anything).Return("We were unable to process your request, please try again")
				mockAuth.On("AuthenticatedWithUac", mock.Anything).Return()
				mockAuth.On("NotAuthWithError", mock.Anything, mock.Anything).Return()
				mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(nil, errors.New("No JWT"))

				httpRecorder = CreateTestResponseRecorder()
				req, _ := http.NewRequest("GET", fmt.Sprintf("/%s/", instrumentName), nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			It("Returns a 401 response with an internal server error", func() {
				mockAuth.AssertNumberOfCalls(GinkgoT(), "NotAuthWithError", 1)

				Expect(observedLogs.Len()).To(Equal(1))
				Expect(observedLogs.All()[0].Message).To(Equal("Error decrypting JWT"))
				Expect(observedLogs.All()[0].ContextMap()["error"]).To(Equal("No JWT"))
				Expect(observedLogs.All()[0].Level).To(Equal(zap.ErrorLevel))
			})
		})

		Context("Blaise returns a non 200 status code", func() {
			JustBeforeEach(func() {
				languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
				httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/default.aspx", catiUrl, instrumentName),
					httpmock.NewJsonResponderOrPanic(500, "Sad face"))

				mockAuth.On("AuthenticatedWithUac", mock.Anything).Return()
				mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{UacInfo: busapi.UacInfo{
					InstrumentName: instrumentName,
					CaseID:         caseID,
				}}, nil)

				httpRecorder = CreateTestResponseRecorder()
				req, _ := http.NewRequest("GET", fmt.Sprintf("/%s/", instrumentName), nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			It("return a 500 error and redirect to the internal server error page", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusInternalServerError))
				Expect(httpRecorder.Body.String()).To(ContainSubstring("Sorry, there is a problem with the service"))

				Expect(observedLogs.Len()).To(Equal(1))
				Expect(observedLogs.All()[0].Message).To(Equal("Error launching blaise study, invalid status code"))
				Expect(observedLogs.All()[0].ContextMap()["AuthedCaseID"]).To(Equal(caseID))
				Expect(observedLogs.All()[0].ContextMap()["AuthedInstrumentName"]).To(Equal(instrumentName))
				Expect(observedLogs.All()[0].ContextMap()["RespStatusCode"]).To(Equal(int64(500)))
				Expect(observedLogs.All()[0].ContextMap()["RespBody"]).To(Equal(`"Sad face"`))
				Expect(observedLogs.All()[0].Level).To(Equal(zap.ErrorLevel))
			})
		})
	})

	Describe("Proxy get requests to blaise", func() {
		Context("Making a request for a blaise resource gets proxied to the blaise server", func() {
			JustBeforeEach(func() {
				httpmock.RegisterResponder("GET", fmt.Sprintf("%s/%s/fwibble/dwibble/qwibble", catiUrl, instrumentName),
					httpmock.NewStringResponder(200, responseInfo))

				mockAuth.On("AuthenticatedWithUac", mock.Anything).Return()
				mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{UacInfo: busapi.UacInfo{
					InstrumentName: instrumentName,
					CaseID:         caseID,
				}}, nil)

				httpRecorder = CreateTestResponseRecorder()
				req, _ := http.NewRequest("GET", fmt.Sprintf("/%s/fwibble/dwibble/qwibble", instrumentName), nil)
				req.Header.Add("Content-Type", "application/json")
				req.Header.Add("Connection", "foobar")
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			It("Returns a 200 response and some data", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusOK))
				Expect(httpRecorder.Body.String()).To(ContainSubstring(responseInfo))
			})
		})

		Context("Making a request for a blaise resource gets proxied to the blaise server for short urls", func() {
			JustBeforeEach(func() {
				httpmock.RegisterResponder("GET", fmt.Sprintf("%s/%s/fwibble", catiUrl, instrumentName),
					httpmock.NewStringResponder(200, responseInfo))

				mockAuth.On("AuthenticatedWithUac", mock.Anything).Return()
				mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{UacInfo: busapi.UacInfo{
					InstrumentName: instrumentName,
					CaseID:         caseID,
				}}, nil)

				httpRecorder = CreateTestResponseRecorder()
				req, _ := http.NewRequest("GET", fmt.Sprintf("/%s/fwibble", instrumentName), nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			It("Returns a 200 response and some data", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusOK))
				Expect(httpRecorder.Body.String()).To(ContainSubstring(responseInfo))
			})
		})

		Context("When the get is for a different instrument", func() {
			JustBeforeEach(func() {
				languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
				httpmock.RegisterResponder("GET", fmt.Sprintf("%s/%s/fwibble", catiUrl, "notMyInstrument"),
					httpmock.NewStringResponder(200, responseInfo))

				mockAuth.On("AuthenticatedWithUac", mock.Anything).Return()
				mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{UacInfo: busapi.UacInfo{
					InstrumentName: instrumentName,
					CaseID:         caseID,
				}}, nil)

				httpRecorder = CreateTestResponseRecorder()
				req, _ := http.NewRequest("GET", fmt.Sprintf("/%s/fwibble", "notMyInstrument"), nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			It("gets wrapped by a http forbidden error", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusForbidden))
				Expect(httpRecorder.Body.String()).To(ContainSubstring(
					`To access this page you need to <a href="/">re-enter your access code</a>`,
				))
			})
		})
	})

	Describe("Proxy post requests to blaise", func() {
		Context("Making a request for a blaise resource posts proxied to the blaise server", func() {
			JustBeforeEach(func() {
				httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/fwibble", catiUrl, instrumentName),
					httpmock.NewStringResponder(200, responseInfo))

				mockAuth.On("AuthenticatedWithUac", mock.Anything).Return()
				mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{UacInfo: busapi.UacInfo{
					InstrumentName: instrumentName,
					CaseID:         caseID,
				}}, nil)

				requestBody = bytes.NewReader([]byte(`{"foo":"bar"}`))

				httpRecorder = CreateTestResponseRecorder()
				req, _ := http.NewRequest("POST", fmt.Sprintf("/%s/fwibble", instrumentName), requestBody)
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			It("Returns a 200 response and some data", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusOK))
				Expect(httpRecorder.Body.String()).To(ContainSubstring(responseInfo))
			})
		})

		Context("Making a request to start interview via POST to Blaise server", func() {
			var requestedCaseID string

			JustBeforeEach(func() {
				httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/api/application/start_interview", catiUrl, instrumentName),
					httpmock.NewStringResponder(200, responseInfo))

				mockAuth.On("AuthenticatedWithUac", mock.Anything).Return()
				mockJWTCrypto.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{UacInfo: busapi.UacInfo{
					InstrumentName: instrumentName,
					CaseID:         caseID,
				}}, nil)

				requestBody = bytes.NewReader([]byte(fmt.Sprintf(`{
						"RuntimeParameters": {
							"KeyValue": "%s",
							"Mode": "CAWI"
						}
					}`, requestedCaseID)))

				httpRecorder = CreateTestResponseRecorder()
				req, _ := http.NewRequest("POST", fmt.Sprintf("/%s/api/application/start_interview", instrumentName), requestBody)
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			Context("When the case ID has authorisation", func() {
				BeforeEach(func() {
					requestedCaseID = caseID
				})

				It("Returns a 200 response and some data", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusOK))
					Expect(httpRecorder.Body.String()).To(ContainSubstring(responseInfo))
				})
			})

			Context("When the case ID does not have authorisation", func() {
				BeforeEach(func() {
					languageManagerMock.On("IsWelsh", mock.Anything).Return(false)
					requestedCaseID = "notMyCaseID"
				})

				It("Returns a 200 response and some data", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusForbidden))
					Expect(httpRecorder.Body.String()).To(ContainSubstring(
						`To access this page you need to <a href="/">re-enter your access code</a>`,
					))

					Expect(observedLogs.Len()).To(Equal(1))
					Expect(observedLogs.All()[0].Message).To(Equal("Not authenticated to start interview for case"))
					Expect(observedLogs.All()[0].ContextMap()["AuthedCaseID"]).To(Equal(caseID))
					Expect(observedLogs.All()[0].ContextMap()["AuthedInstrumentName"]).To(Equal(instrumentName))
					Expect(observedLogs.All()[0].ContextMap()["CaseID"]).To(Equal(requestedCaseID))
					Expect(observedLogs.All()[0].Level).To(Equal(zap.InfoLevel))
				})
			})
		})
	})
})

var _ = Describe("GET /:instrumentName/logout", func() {
	var (
		httpRouter           *gin.Engine
		mockAuth             = &mocks.AuthInterface{}
		instrumentController = &webserver.InstrumentController{Auth: mockAuth}
		instrumentName       = "foobar"
	)

	BeforeEach(func() {
		httpRouter = gin.Default()
		store := cookie.NewStore([]byte("secret"))
		httpRouter.Use(sessions.SessionsMany([]string{"session", "user_session", "session_validation"}, store))
		httpRouter.SetFuncMap(template.FuncMap{
			"WrapWelsh": webserver.WrapWelsh,
		})
		httpRouter.LoadHTMLGlob("../templates/*")
		instrumentController.AddRoutes(httpRouter)
	})

	AfterEach(func() {
		mockAuth = &mocks.AuthInterface{}
		instrumentController = &webserver.InstrumentController{Auth: mockAuth}
	})

	var (
		httpRecorder *TestResponseRecorder
	)

	BeforeEach(func() {
		mockAuth.On("Logout", mock.Anything, mock.Anything).Return()
	})

	JustBeforeEach(func() {
		httpRecorder = CreateTestResponseRecorder()
		req, _ := http.NewRequest("GET", fmt.Sprintf("/%s/logout", instrumentName), nil)
		httpRouter.ServeHTTP(httpRecorder, req)
	})

	It("calls it auth.logout", func() {
		mockAuth.AssertNumberOfCalls(GinkgoT(), "Logout", 1)
	})
})
