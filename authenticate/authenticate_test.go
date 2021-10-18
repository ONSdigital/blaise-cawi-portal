package authenticate_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	mockauth "github.com/ONSdigital/blaise-cawi-portal/authenticate/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/ONSdigital/blaise-cawi-portal/busapi/mocks"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Login", func() {
	var (
		shortUAC   = "22222"
		longUAC    = "11112222333344445555"
		spacedUAC   = "1234 5678 9012"
		spacedUAC16 = "bcdf 5678 ghjk 2345"
		validUAC    = "123456789012"
		validUAC16  = "bcdf5678ghjk2345"
		jwtCrypto   = &authenticate.JWTCrypto{
			JWTSecret: "hello",
		}
		auth = &authenticate.Auth{
			JWTCrypto: jwtCrypto,
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
			})
		})

		Context("Login with a 16 digit UAC kind", func() {
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

		Context("Login with a 12 digit UAC kind", func(){
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
			})
		})

		Context("Login with a 16 digit UAC kind", func(){
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

		Context("Login with a 12 digit UAC kind", func(){
			BeforeEach(func() {
				auth.UacKind = "uac"
			})

			It("returns a status unauthorised with an error", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
				Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
				Expect(session.Get(authenticate.JWT_TOKEN_KEY)).To(BeNil())
				body := httpRecorder.Body.Bytes()
				Expect(strings.Contains(string(body), `Enter a 12-character access code`)).To(BeTrue())
			})
		})

		Context("Login with a 16 digit UAC kind", func(){
			BeforeEach(func() {
				auth.UacKind = "uac16"
			})

			It("returns a status unauthorised with an error", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
				Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
				Expect(session.Get(authenticate.JWT_TOKEN_KEY)).To(BeNil())
				body := httpRecorder.Body.Bytes()
				Expect(strings.Contains(string(body), `Enter a 16-character access code`)).To(BeTrue())
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
			httpRouter.ServeHTTP(httpRecorder, req)
		})

		Context("Login with a 12 digit UAC kind", func(){
			BeforeEach(func() {
				auth.UacKind = "uac"
			})

			It("returns a status unauthorised with an error", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
				Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
				Expect(session.Get(authenticate.JWT_TOKEN_KEY)).To(BeNil())
				body := httpRecorder.Body.Bytes()
				Expect(strings.Contains(string(body), `Enter a 12-character access code`)).To(BeTrue())
			})
		})

		Context("Login with a 16 digit UAC kind", func(){
			BeforeEach(func() {
				auth.UacKind = "uac16"
			})

			It("returns a status unauthorised with an error", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
				Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
				Expect(session.Get(authenticate.JWT_TOKEN_KEY)).To(BeNil())
				body := httpRecorder.Body.Bytes()
				Expect(strings.Contains(string(body), `Enter a 16-character access code`)).To(BeTrue())
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
			httpRouter.ServeHTTP(httpRecorder, req)
		})

		Context("Login with a 12 digit UAC kind", func(){
			BeforeEach(func() {
				auth.UacKind = "uac"
			})

			It("returns a status unauthorised with an error", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
				Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
				Expect(session.Get(authenticate.JWT_TOKEN_KEY)).To(BeNil())
				body := httpRecorder.Body.Bytes()
				Expect(strings.Contains(string(body), `Enter an access code`)).To(BeTrue())
			})
		})

		Context("Login with a 16 digit UAC kind", func(){
			BeforeEach(func() {
				auth.UacKind = "uac16"
			})

			It("returns a status unauthorised with an error", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
				Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
				Expect(session.Get(authenticate.JWT_TOKEN_KEY)).To(BeNil())
				body := httpRecorder.Body.Bytes()
				Expect(strings.Contains(string(body), `Enter an access code`)).To(BeTrue())
			})
		})
	})
})

var _ = Describe("Logout", func() {
	var (
		httpRouter   *gin.Engine
		httpRecorder *httptest.ResponseRecorder
		session      sessions.Session
		auth         = &authenticate.Auth{}
	)

	BeforeEach(func() {
		httpRouter = gin.Default()
		httpRouter.LoadHTMLGlob("../templates/*")
		store := cookie.NewStore([]byte("secret"))
		httpRouter.Use(sessions.Sessions("mysession", store))
		httpRouter.GET("/logout", func(context *gin.Context) {
			session = sessions.Default(context)
			session.Set("foobar", "fizzbuzz")
			session.Save()
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

var _ = Describe("AuthenticatedWithUac", func() {
	var (
		session sessions.Session

		mockJwtCrypto = &mockauth.JWTCryptoInterface{}
		auth          = &authenticate.Auth{
			JWTCrypto: mockJwtCrypto,
		}
		httpRecorder *httptest.ResponseRecorder
		httpRouter   *gin.Engine
	)

	BeforeEach(func() {
		httpRouter = gin.Default()
		httpRouter.LoadHTMLGlob("../templates/*")
		store := cookie.NewStore([]byte("secret"))
		httpRouter.Use(sessions.Sessions("mysession", store))
	})

	AfterEach(func() {
		httpRouter = gin.Default()
		mockJwtCrypto = &mockauth.JWTCryptoInterface{}
		auth.JWTCrypto = mockJwtCrypto
	})

	JustBeforeEach(func() {
		httpRecorder = httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		httpRouter.ServeHTTP(httpRecorder, req)
	})

	Context("when there is a token", func() {
		BeforeEach(func() {
			httpRouter.Use(func(context *gin.Context) {
				session = sessions.Default(context)
				session.Set(authenticate.JWT_TOKEN_KEY, "foobar")
				session.Save()
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

			It("Allows the context to continue", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusOK))
				body := httpRecorder.Body.Bytes()
				Expect(string(body)).To(Equal("true"))
			})
		})

		Context("When a token cannot be decrypted", func() {
			BeforeEach(func() {
				mockJwtCrypto.On("DecryptJWT", mock.Anything).Return(nil, fmt.Errorf("Explosions"))
			})

			It("return unauthorized", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
				body := httpRecorder.Body.Bytes()
				Expect(strings.Contains(string(body), `<span class="btn__inner">Access survey`)).To(BeTrue())
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

		It("return unauthorized", func() {
			Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
			body := httpRecorder.Body.Bytes()
			Expect(strings.Contains(string(body), `<span class="btn__inner">Access survey`)).To(BeTrue())
		})
	})
})

var _ = Describe("Has Session", func() {
	var (
		session sessions.Session

		mockJwtCrypto = &mockauth.JWTCryptoInterface{}
		auth          = &authenticate.Auth{
			JWTCrypto: mockJwtCrypto,
		}
		httpRecorder   *httptest.ResponseRecorder
		httpRouter     *gin.Engine
		instrumentName = "foobar"
		caseID         = "fizzbuzz"
	)

	BeforeEach(func() {
		httpRouter = gin.Default()
		httpRouter.LoadHTMLGlob("../templates/*")
		store := cookie.NewStore([]byte("secret"))
		httpRouter.Use(sessions.Sessions("mysession", store))

		httpRouter.Use(func(context *gin.Context) {
			session = sessions.Default(context)
			session.Set(authenticate.JWT_TOKEN_KEY, "foobar")
			session.Save()
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
				`{"HasSession":true,"Claim":{"uac":"","instrument_name":"foobar","case_id":"fizzbuzz"}}`,
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
