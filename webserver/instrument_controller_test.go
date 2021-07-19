package webserver_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/authenticate/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/ONSdigital/blaise-cawi-portal/webserver"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

var _ = Describe("Open Case", func() {
	var (
		catiUrl              = "http://localhost"
		instrumentName       = "foobar"
		caseID               = "fizzbuzz"
		httpRouter           *gin.Engine
		httpRecorder         *httptest.ResponseRecorder
		responseInfo         = "hello"
		mockAuth             *mocks.AuthInterface
		instrumentController = &webserver.InstrumentController{CatiUrl: catiUrl, HttpClient: &http.Client{}}
		requestBody          io.Reader
	)

	BeforeEach(func() {
		httpRouter = gin.Default()
		httpRouter.LoadHTMLGlob("../templates/*")
		store := cookie.NewStore([]byte("secret"))
		httpRouter.Use(sessions.Sessions("mysession", store))
		instrumentController.AddRoutes(httpRouter)
		httpmock.Activate()
	})

	AfterEach(func() {
		httpmock.DeactivateAndReset()
	})

	Describe("Open a case in Blaise", func() {
		Context("Launching Blaise in Cawi mode with a valid instrument and case id", func() {
			JustBeforeEach(func() {
				httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/default.aspx", catiUrl, instrumentName),
					httpmock.NewJsonResponderOrPanic(200, responseInfo))

				mockAuth = &mocks.AuthInterface{}
				mockAuth.On("Authenticated", mock.Anything).Return()
				mockAuth.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{UacInfo: busapi.UacInfo{
					InstrumentName: instrumentName,
					CaseID:         caseID,
				}}, nil)
				instrumentController.Auth = mockAuth

				httpRecorder = httptest.NewRecorder()
				req, _ := http.NewRequest("GET", fmt.Sprintf("/%s/", instrumentName), nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			It("Returns a 200 response and some data", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusOK))
				body := httpRecorder.Body.Bytes()
				Expect(strings.Contains(string(body), responseInfo)).To(BeTrue())
			})
		})

		Context("Launching Blaise in Cawi mode for a different instrument", func() {
			JustBeforeEach(func() {
				httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/default.aspx", catiUrl, instrumentName),
					httpmock.NewJsonResponderOrPanic(200, responseInfo))

				mockAuth = &mocks.AuthInterface{}
				mockAuth.On("Authenticated", mock.Anything).Return()
				mockAuth.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{UacInfo: busapi.UacInfo{
					InstrumentName: instrumentName,
					CaseID:         caseID,
				}}, nil)
				instrumentController.Auth = mockAuth

				httpRecorder = httptest.NewRecorder()
				req, _ := http.NewRequest("GET", fmt.Sprintf("/%s/", "fwibble"), nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			It("Returns a 403", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusForbidden))
				// body := httpRecorder.Body.Bytes()
				// Expect(strings.Contains(string(body), responseInfo)).To(BeTrue())
			})
		})

		Context("Invalid responses from opening a case in Blaise", func() {
			JustBeforeEach(func() {
				mockAuth = &mocks.AuthInterface{}
				mockAuth.On("Authenticated", mock.Anything).Return()
				mockAuth.On("DecryptJWT", mock.Anything).Return(nil, errors.New("No JWT"))
				instrumentController.Auth = mockAuth

				httpRecorder = httptest.NewRecorder()
				req, _ := http.NewRequest("GET", fmt.Sprintf("/%s/", instrumentName), nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			It("Returns a 401 response with an internal server error", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
				body := httpRecorder.Body.Bytes()
				Expect(strings.Contains(string(body), authenticate.INTERNAL_SERVER_ERR)).To(BeTrue())
			})
		})

		Context("Blaise returns a non 200 status code", func() {
			JustBeforeEach(func() {
				httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/default.aspx", catiUrl, instrumentName),
					httpmock.NewJsonResponderOrPanic(500, "Sad face"))

				mockAuth = &mocks.AuthInterface{}
				mockAuth.On("Authenticated", mock.Anything).Return()
				mockAuth.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{UacInfo: busapi.UacInfo{
					InstrumentName: instrumentName,
					CaseID:         caseID,
				}}, nil)
				instrumentController.Auth = mockAuth

				httpRecorder = httptest.NewRecorder()
				req, _ := http.NewRequest("GET", fmt.Sprintf("/%s/", instrumentName), nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			It("return a 500 error and redirect to the internal server error page", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusInternalServerError))
				body := httpRecorder.Body.Bytes()
				Expect(strings.Contains(string(body), "Sorry, there is a problem with the service")).To(BeTrue())
			})
		})
	})

	Describe("Proxy get requests to blaise", func() {
		Context("Making a request for a blaise resource gets proxied to the blaise server", func() {
			JustBeforeEach(func() {
				httpmock.RegisterResponder("GET", fmt.Sprintf("%s/%s/fwibble/dwibble/qwibble", catiUrl, instrumentName),
					httpmock.NewJsonResponderOrPanic(200, responseInfo))

				mockAuth = &mocks.AuthInterface{}
				mockAuth.On("Authenticated", mock.Anything).Return()
				mockAuth.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{UacInfo: busapi.UacInfo{
					InstrumentName: instrumentName,
					CaseID:         caseID,
				}}, nil)
				instrumentController.Auth = mockAuth

				httpRecorder = httptest.NewRecorder()
				req, _ := http.NewRequest("GET", fmt.Sprintf("/%s/fwibble/dwibble/qwibble", instrumentName), nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			It("Returns a 200 response and some data", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusOK))
				body := httpRecorder.Body.Bytes()
				Expect(strings.Contains(string(body), responseInfo)).To(BeTrue())
			})
		})

		Context("Making a request for a blaise resource gets proxied to the blaise server for short urls", func() {
			JustBeforeEach(func() {
				httpmock.RegisterResponder("GET", fmt.Sprintf("%s/%s/fwibble", catiUrl, instrumentName),
					httpmock.NewJsonResponderOrPanic(200, responseInfo))

				mockAuth = &mocks.AuthInterface{}
				mockAuth.On("Authenticated", mock.Anything).Return()
				mockAuth.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{UacInfo: busapi.UacInfo{
					InstrumentName: instrumentName,
					CaseID:         caseID,
				}}, nil)
				instrumentController.Auth = mockAuth

				httpRecorder = httptest.NewRecorder()
				req, _ := http.NewRequest("GET", fmt.Sprintf("/%s/fwibble", instrumentName), nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			It("Returns a 200 response and some data", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusOK))
				body := httpRecorder.Body.Bytes()
				Expect(strings.Contains(string(body), responseInfo)).To(BeTrue())
			})
		})

		Describe("Proxy post requests to blaise", func() {
			Context("Making a request for a blaise resource posts proxied to the blaise server", func() {
				JustBeforeEach(func() {
					httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/fwibble", catiUrl, instrumentName),
						httpmock.NewJsonResponderOrPanic(200, responseInfo))

					mockAuth = &mocks.AuthInterface{}
					mockAuth.On("Authenticated", mock.Anything).Return()
					mockAuth.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{UacInfo: busapi.UacInfo{
						InstrumentName: instrumentName,
						CaseID:         caseID,
					}}, nil)
					instrumentController.Auth = mockAuth

					requestBody = bytes.NewReader([]byte(`{"foo":"bar"}`))

					httpRecorder = httptest.NewRecorder()
					req, _ := http.NewRequest("POST", fmt.Sprintf("/%s/fwibble", instrumentName), requestBody)
					httpRouter.ServeHTTP(httpRecorder, req)
				})

				It("Returns a 200 response and some data", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusOK))
					body := httpRecorder.Body.Bytes()
					Expect(strings.Contains(string(body), responseInfo)).To(BeTrue())
				})
			})

			Context("Making a request to start interview via POST to Blaise server", func() {
				JustBeforeEach(func() {
					httpmock.RegisterResponder("POST", fmt.Sprintf("%s/%s/api/application/start_interview", catiUrl, instrumentName),
						httpmock.NewJsonResponderOrPanic(200, responseInfo))

					mockAuth = &mocks.AuthInterface{}
					mockAuth.On("Authenticated", mock.Anything).Return()
					mockAuth.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{UacInfo: busapi.UacInfo{
						InstrumentName: instrumentName,
						CaseID:         caseID,
					}}, nil)
					instrumentController.Auth = mockAuth

					requestBody = bytes.NewReader([]byte(fmt.Sprintf(`{
						"RuntimeParameters": {
							"KeyValue": "%s",
							"Mode": "CAWI",
							"LayoutSet": "CAWI-Web_Large"
						}
					}`, caseID)))

					httpRecorder = httptest.NewRecorder()
					req, _ := http.NewRequest("POST", fmt.Sprintf("/%s/api/application/start_interview", instrumentName), requestBody)
					httpRouter.ServeHTTP(httpRecorder, req)
				})

				It("Returns a 200 response and some data", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusOK))
					body := httpRecorder.Body.Bytes()
					Expect(strings.Contains(string(body), responseInfo)).To(BeTrue())
				})
			})
		})
	})
})
