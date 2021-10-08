package webserver_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/authenticate/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/ONSdigital/blaise-cawi-portal/webserver"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/stretchr/testify/mock"
	csrf "github.com/utrack/gin-csrf"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Auth Controller", func() {
	var (
		httpRouter     *gin.Engine
		mockAuth       = &mocks.AuthInterface{}
		authController = &webserver.AuthController{
			Auth: mockAuth,
			CSRFSecret: "fwibble",
			UacKind: "uac"}
		instrumentName = "foobar"
		caseID         = "fizzbuzz"
	)

	BeforeEach(func() {
		httpRouter = gin.Default()
		store := cookie.NewStore([]byte("secret"))
		httpRouter.Use(sessions.Sessions("mysession", store))
		httpRouter.LoadHTMLGlob("../templates/*")
		authController.AddRoutes(httpRouter)
	})

	AfterEach(func() {
		mockAuth = &mocks.AuthInterface{}
		authController = &webserver.AuthController{Auth: mockAuth}
	})

	Describe("GET /auth/login", func() {
		var (
			httpRecorder *httptest.ResponseRecorder
		)

		JustBeforeEach(func() {
			httpRecorder = httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/auth/login", nil)
			httpRouter.ServeHTTP(httpRecorder, req)
		})

		Context("when I access auth/login I am presented with the login template", func() {
			BeforeEach(func() {
				mockAuth.On("HasSession", mock.Anything).Return(false, nil)
			})

			It("returns a 200 response and the login page", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusOK))
				Expect(httpRecorder.Body.String()).To(ContainSubstring(`<span class="btn__inner">Access survey`))
			})
		})

		Context("when I access auth/login with an active session", func() {
			JustBeforeEach(func() {
				httpRecorder = httptest.NewRecorder()

				mockAuth.On("HasSession", mock.Anything).Return(true, &authenticate.UACClaims{UacInfo: busapi.UacInfo{
					InstrumentName: instrumentName,
					CaseID:         caseID,
				}}, nil)

				req, _ := http.NewRequest("GET", "/auth/login", nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			It("returns a temporary redirect response", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusTemporaryRedirect))

				header := httpRecorder.Header()["Location"]
				Expect(header).To(Equal([]string{fmt.Sprintf("/%s/", instrumentName)}))
			})
		})
	})

	Describe("POST /auth/login", func() {
		var (
			httpRecorder *httptest.ResponseRecorder
		)

		BeforeEach(func() {
			mockAuth.On("Login", mock.Anything, mock.Anything).Return()
		})

		Context("without a CSRF", func() {
			JustBeforeEach(func() {
				httpRecorder = httptest.NewRecorder()
				req, _ := http.NewRequest("POST", "/auth/login", nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			It("gives an auth error", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusForbidden))
				Expect(httpRecorder.Body.String()).To(ContainSubstring(`<strong>Something went wrong`))
			})
		})

		Context("with an invalid CSRF", func() {
			JustBeforeEach(func() {
				httpRecorder = httptest.NewRecorder()
				req, _ := http.NewRequest("POST", "/auth/login?_csrf=dalajksdqoosk", nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})

			It("gives an auth error", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusForbidden))
				Expect(httpRecorder.Body.String()).To(ContainSubstring(`<strong>Something went wrong`))
			})
		})

		Context("with a valid CSRF", func() {
			var csrfToken string

			JustBeforeEach(func() {
				httpRouter.GET("/token", func(context *gin.Context) {
					context.Set("csrfSecret", authController.CSRFSecret)
					csrfToken = csrf.GetToken(context)
				})

				req1, _ := http.NewRequest("GET", "/token", nil)

				httpRecorder = httptest.NewRecorder()
				httpRouter.ServeHTTP(httpRecorder, req1)

				req2, _ := http.NewRequest("POST", fmt.Sprintf("/auth/login?_csrf=%s", csrfToken), nil)
				req2.Header.Set("Cookie", httpRecorder.Header().Get("Set-Cookie"))
				req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")

				httpRecorder = httptest.NewRecorder()
				httpRouter.ServeHTTP(httpRecorder, req2)
			})

			It("calls it auth.login", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusOK))
				mockAuth.AssertNumberOfCalls(GinkgoT(), "Login", 1)
			})
		})
	})

	Describe("GET /auth/logout", func() {
		var (
			httpRecorder *httptest.ResponseRecorder
		)

		BeforeEach(func() {
			mockAuth.On("Logout", mock.Anything, mock.Anything).Return()
		})

		JustBeforeEach(func() {
			httpRecorder = httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/auth/logout", nil)
			httpRouter.ServeHTTP(httpRecorder, req)
		})

		It("calls it auth.logout", func() {
			mockAuth.AssertNumberOfCalls(GinkgoT(), "Logout", 1)
		})
	})
})
