package languagemanager_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/ONSdigital/blaise-cawi-portal/languagemanager"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LanguageManager", func() {
	var lanauageManager = &languagemanager.Manager{SessionName: "language_session"}
	Describe("IsWelsh", func() {
		var (
			httpRecorder *httptest.ResponseRecorder
			httpRouter   *gin.Engine
		)

		BeforeEach(func() {
			httpRecorder = httptest.NewRecorder()

			httpRouter = gin.Default()
			store := cookie.NewStore([]byte("secret"))
			httpRouter.Use(sessions.SessionsMany([]string{"language_session"}, store))
		})

		Context("when the session property of welsh is true", func() {

			It("returns true", func() {
				httpRouter.GET("/", func(context *gin.Context) {
					session := sessions.DefaultMany(context, "language_session")
					session.Set("welsh", true)
					session.Save()
					Expect(lanauageManager.IsWelsh(context)).To(BeTrue())
					context.Status(200)
				})

				req, _ := http.NewRequest("GET", "/", nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})
		})

		Context("when the session property of welsh is false", func() {
			It("returns false", func() {
				httpRouter.GET("/", func(context *gin.Context) {
					session := sessions.DefaultMany(context, "language_session")
					session.Set("welsh", false)
					session.Save()
					Expect(lanauageManager.IsWelsh(context)).To(BeFalse())
				})

				req, _ := http.NewRequest("GET", "/", nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})
		})

		Context("when the session property of welsh is unset", func() {
			It("returns false", func() {
				httpRouter.GET("/", func(context *gin.Context) {
					Expect(lanauageManager.IsWelsh(context)).To(BeFalse())
				})

				req, _ := http.NewRequest("GET", "/", nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})
		})

		Context("when the session property of welsh is not a bool", func() {
			It("returns false", func() {
				httpRouter.GET("/", func(context *gin.Context) {
					session := sessions.DefaultMany(context, "language_session")
					session.Set("welsh", "foo")
					session.Save()
					Expect(lanauageManager.IsWelsh(context)).To(BeFalse())
				})

				req, _ := http.NewRequest("GET", "/", nil)
				httpRouter.ServeHTTP(httpRecorder, req)
			})
		})
	})
})
