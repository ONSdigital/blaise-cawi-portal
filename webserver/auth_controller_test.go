package webserver_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/webserver"

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
		httpRouter.LoadHTMLGlob("../templates/*")
		authController.AddRoutes(httpRouter)
	})

	Describe("/auth/login", func() {
		var (
			httpRecorder *httptest.ResponseRecorder
		)

		JustBeforeEach(func() {
			httpRecorder = httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/auth/login", nil)
			httpRouter.ServeHTTP(httpRecorder, req)
		})

		Context("when I access auth/login I am presented with the login template", func() {
			It("returns a 200 response and the login page", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusOK))
				body := httpRecorder.Body.Bytes()
				fmt.Println("WTF IS HAPPENING IN THE WORLD")
				fmt.Println(string(body))
				Expect(strings.Contains(string(body), `<span class="btn__inner">Access survey</span>`)).To(BeTrue())
			})
		})
	})
})
