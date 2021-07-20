package webserver_test

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/webserver"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/stretchr/testify/mock"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Auth Controller", func() {
	var (
		httpRouter     *gin.Engine
		authController = &webserver.AuthController{}
	)

	BeforeEach(func() {
		httpRouter = gin.Default()
		store := cookie.NewStore([]byte("secret"))
		httpRouter.Use(sessions.Sessions("mysession", store))
		httpRouter.LoadHTMLGlob("../templates/*")
		authController.AddRoutes(httpRouter)
	})

	Describe("GET /auth/login", func() {
		var (
			mockAuth     *mocks.AuthInterface
			httpRecorder *httptest.ResponseRecorder
		)

		BeforeEach(func() {
			mockAuth = &mocks.AuthInterface{}
			mockAuth.On("HasSession", mock.Anything).Return(false, nil)
			authController.Auth = mockAuth
		})

		JustBeforeEach(func() {
			httpRecorder = httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/auth/login", nil)
			httpRouter.ServeHTTP(httpRecorder, req)
		})

		Context("when I access auth/login I am presented with the login template", func() {
			It("returns a 200 response and the login page", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusOK))
				body := httpRecorder.Body.Bytes()
				Expect(strings.Contains(string(body), `<span class="btn__inner">Access survey</span>`)).To(BeTrue())
			})
		})
	})

	Describe("POST /auth/login", func() {
		var (
			mockAuth     *mocks.AuthInterface
			httpRecorder *httptest.ResponseRecorder
		)

		BeforeEach(func() {
			mockAuth = &mocks.AuthInterface{}
			mockAuth.On("Login", mock.Anything, mock.Anything).Return()
			authController.Auth = mockAuth
		})

		JustBeforeEach(func() {
			httpRecorder = httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/auth/login", nil)
			httpRouter.ServeHTTP(httpRecorder, req)
		})

		It("calls it auth.login", func() {
			mockAuth.AssertNumberOfCalls(GinkgoT(), "Login", 1)
		})
	})

	Describe("GET /auth/logout", func() {
		var (
			mockAuth     *mocks.AuthInterface
			httpRecorder *httptest.ResponseRecorder
		)

		BeforeEach(func() {
			mockAuth = &mocks.AuthInterface{}
			mockAuth.On("Logout", mock.Anything, mock.Anything).Return()
			authController.Auth = mockAuth
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
