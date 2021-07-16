package login_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/ONSdigital/blaise-cawi-portal/busapi/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/login"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Login", func() {
	var (
		shortUAC = "22222"
		longUAC  = "1111222233334444"
		validUAC = "123456789012"
		auth     = &login.Auth{
			JWTSecret: "hello",
		}
		httpRouter   *gin.Engine
		httpRecorder *httptest.ResponseRecorder
		session      sessions.Session
	)

	BeforeEach(func() {
		httpRouter = gin.Default()
		httpRouter.LoadHTMLGlob("../templates/*")
		store := cookie.NewStore([]byte("secret"))
		httpRouter.Use(sessions.Sessions("mysession", store))
		httpRouter.POST("/login", func(context *gin.Context) {
			session = sessions.Default(context)
			auth.Login(context, session)
		})
	})

	Context("Login with a valid UAC Code", func() {
		BeforeEach(func() {
			mockBusApi := &mocks.BusApiInterface{}
			auth.BusApi = mockBusApi

			mockBusApi.On("GetUacInfo", validUAC).Once().Return(busapi.UacInfo{InstrumentName: "foo", CaseID: "bar"}, nil)
		})

		JustBeforeEach(func() {
			httpRecorder = httptest.NewRecorder()
			data := url.Values{
				"uac": []string{validUAC},
			}
			req, _ := http.NewRequest("POST", "/login", strings.NewReader(data.Encode()))
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			httpRouter.ServeHTTP(httpRecorder, req)
		})

		It("redirects to /:instrumentName/", func() {
			Expect(httpRecorder.Code).To(Equal(http.StatusMovedPermanently))
			Expect(httpRecorder.Header()["Location"]).To(Equal([]string{"/foo/"}))
			Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
			decryptedToken, _ := auth.DecryptJWT(session.Get(login.JWT_TOKEN_KEY))
			Expect(decryptedToken.UAC).To(Equal(validUAC))
			Expect(decryptedToken.UacInfo.InstrumentName).To(Equal("foo"))
			Expect(decryptedToken.UacInfo.CaseID).To(Equal("bar"))
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

		It("returns a status unauthorised with an error", func() {
			Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
			Expect(httpRecorder.Result().Cookies()).To(BeEmpty())
			body := httpRecorder.Body.Bytes()
			Expect(strings.Contains(string(body), `Enter a 12-character access code`))
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
			httpRouter.ServeHTTP(httpRecorder, req)
		})

		It("returns a status unauthorised with an error", func() {
			Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
			Expect(httpRecorder.Result().Cookies()).To(BeEmpty())
			body := httpRecorder.Body.Bytes()
			Expect(strings.Contains(string(body), `Enter a 12-character access code`))
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
			httpRouter.ServeHTTP(httpRecorder, req)
		})

		It("returns a status unauthorised with an error", func() {
			Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
			Expect(httpRecorder.Result().Cookies()).To(BeEmpty())
			body := httpRecorder.Body.Bytes()
			Expect(strings.Contains(string(body), `Enter an access code`))
		})
	})
})
