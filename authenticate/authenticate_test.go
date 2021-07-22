package authenticate_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	mockauth "github.com/ONSdigital/blaise-cawi-portal/authenticate/mocks"
	mockrestapi "github.com/ONSdigital/blaise-cawi-portal/blaiserestapi/mocks"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/ONSdigital/blaise-cawi-portal/busapi/mocks"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Login", func() {
	var (
		shortUAC  = "22222"
		longUAC   = "1111222233334444"
		spacedUAC = "1234 5678 9012"
		validUAC  = "123456789012"
		jwtCrypto = &authenticate.JWTCrypto{
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

		It("redirects to /auth/login/postcode", func() {
			Expect(httpRecorder.Code).To(Equal(http.StatusFound))
			Expect(httpRecorder.Header()["Location"]).To(Equal([]string{"/auth/login/postcode"}))
			Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
			decryptedToken, _ := auth.JWTCrypto.DecryptJWT(session.Get(authenticate.JWT_TOKEN_KEY))
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
			Expect(strings.Contains(string(body), `Enter a 12-character access code`)).To(BeTrue())
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
			Expect(strings.Contains(string(body), `Enter a 12-character access code`)).To(BeTrue())
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
				"uac": []string{spacedUAC},
			}
			req, _ := http.NewRequest("POST", "/login", strings.NewReader(data.Encode()))
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			httpRouter.ServeHTTP(httpRecorder, req)
		})

		It("redirects to /auth/login/postcode", func() {
			Expect(httpRecorder.Code).To(Equal(http.StatusFound))
			Expect(httpRecorder.Header()["Location"]).To(Equal([]string{"/auth/login/postcode"}))
			Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
			decryptedToken, _ := auth.JWTCrypto.DecryptJWT(session.Get(authenticate.JWT_TOKEN_KEY))
			Expect(decryptedToken.UAC).To(Equal(validUAC))
			Expect(decryptedToken.UacInfo.InstrumentName).To(Equal("foo"))
			Expect(decryptedToken.UacInfo.CaseID).To(Equal("bar"))
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
			Expect(strings.Contains(string(body), `Enter an access code`)).To(BeTrue())
		})
	})
})

var _ = Describe("LoginPostcode", func() {
	var (
		postcode       string
		uac            = "123456789012"
		instrumentName = "foobar"
		caseID         = "fwibble"
		mockBusApi     = &mocks.BusApiInterface{}
		mockJwtCrypto  = &mockauth.JWTCryptoInterface{}
		mockRestApi    = &mockrestapi.BlaiseRestApiInterface{}
		auth           = &authenticate.Auth{
			JWTCrypto:     mockJwtCrypto,
			BusApi:        mockBusApi,
			BlaiseRestApi: mockRestApi,
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
	})

	AfterEach(func() {
		mockBusApi = &mocks.BusApiInterface{}
		mockJwtCrypto = &mockauth.JWTCryptoInterface{}
		mockRestApi = &mockrestapi.BlaiseRestApiInterface{}
		auth = &authenticate.Auth{
			JWTCrypto:     mockJwtCrypto,
			BusApi:        mockBusApi,
			BlaiseRestApi: mockRestApi,
		}
	})

	JustBeforeEach(func() {
		httpRouter.POST("/login/postcode", func(context *gin.Context) {
			session = sessions.Default(context)
			auth.LoginPostcode(context, session)
		})

		httpRecorder = httptest.NewRecorder()
		data := url.Values{
			"postcode": []string{postcode},
		}
		req, _ := http.NewRequest("POST", "/login/postcode", strings.NewReader(data.Encode()))
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		httpRouter.ServeHTTP(httpRecorder, req)
	})

	Context("when the user has no uac", func() {
		BeforeEach(func() {
			mockBusApi.On("GetUacInfo", uac).Once().Return(busapi.UacInfo{InstrumentName: "foo", CaseID: "bar"}, nil)
		})

		It("returns not authenticated", func() {
			Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
			body := httpRecorder.Body.Bytes()
			Expect(strings.Contains(string(body), `<span class="btn__inner">Access survey</span>`)).To(BeTrue())
		})
	})

	Context("when a /login has already generated a JWT", func() {
		BeforeEach(func() {
			httpRouter.Use(func(context *gin.Context) {
				session = sessions.Default(context)
				session.Set(authenticate.JWT_TOKEN_KEY, "foobar")
				session.Save()
				context.Next()
			})

			mockJwtCrypto.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{
				UAC: uac,
				UacInfo: busapi.UacInfo{
					InstrumentName: instrumentName,
					CaseID:         caseID,
				},
			}, nil)
		})

		Context("when the user does not enter a postcode", func() {
			BeforeEach(func() {
				postcode = ""
				mockRestApi.On("GetPostCode", instrumentName, caseID).Return("not yours", nil)
			})

			It("returns not authenticated", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
				body := httpRecorder.Body.Bytes()
				Expect(strings.Contains(string(body), `Postcode not regognised, please try again`)).To(BeTrue())
			})
		})

		Context("when the user enters a postcode", func() {
			BeforeEach(func() {
				postcode = "NP10 8XG"
			})

			Context("and the postcode does not match", func() {
				BeforeEach(func() {
					mockRestApi.On("GetPostCode", instrumentName, caseID).Return("not yours", nil)
				})

				It("returns not authenticated", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
					body := httpRecorder.Body.Bytes()
					Expect(strings.Contains(string(body), `Postcode not regognised, please try again`)).To(BeTrue())
				})
			})

			Context("and the postcode matches", func() {
				BeforeEach(func() {
					mockRestApi.On("GetPostCode", instrumentName, caseID).Return(postcode, nil)
					mockJwtCrypto.On("EncryptValidatedPostcodeJWT", mock.Anything).Return("signedToken", nil)
				})

				It("redirects to /:instrumentName/", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusFound))
					Expect(httpRecorder.Header()["Location"]).To(Equal([]string{"/foobar/"}))
					Expect(httpRecorder.Result().Cookies()).ToNot(BeEmpty())
					Expect(session.Get(authenticate.JWT_TOKEN_KEY)).To(Equal("signedToken"))
				})
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
				Expect(strings.Contains(string(body), `<span class="btn__inner">Access survey</span>`)).To(BeTrue())
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
			Expect(strings.Contains(string(body), `<span class="btn__inner">Access survey</span>`)).To(BeTrue())
		})
	})
})

var _ = Describe("AuthenticatedWithUacAndPostcode", func() {
	var (
		session sessions.Session

		mockJwtCrypto = &mockauth.JWTCryptoInterface{}
		auth          = &authenticate.Auth{
			JWTCrypto: mockJwtCrypto,
		}
		httpRecorder   *httptest.ResponseRecorder
		httpRouter     *gin.Engine
		instrumentName = "foobar"
		caseID         = "fwibble"
		uac            = "12345678912"
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

			httpRouter.Use(auth.AuthenticatedWithUacAndPostcode)
			httpRouter.GET("/", func(context *gin.Context) {
				context.JSON(200, true)
			})
		})

		Context("When a token can be decrypted", func() {
			Context("and the claim is nil", func() {
				BeforeEach(func() {
					mockJwtCrypto.On("DecryptJWT", mock.Anything).Return(nil, nil)
				})

				It("return unauthorized", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
					body := httpRecorder.Body.Bytes()
					Expect(strings.Contains(string(body), `<span class="btn__inner">Access survey</span>`)).To(BeTrue())
				})
			})

			Context("and the claim does not have postcode validated", func() {
				BeforeEach(func() {
					mockJwtCrypto.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{
						UAC:               uac,
						PostcodeValidated: false,
						UacInfo: busapi.UacInfo{
							InstrumentName: instrumentName,
							CaseID:         caseID,
						},
					}, nil)
				})

				It("return unauthorized", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
					body := httpRecorder.Body.Bytes()
					Expect(strings.Contains(string(body), `<span class="btn__inner">Access survey</span>`)).To(BeTrue())
				})
			})

			Context("and the claim does has postcode validated", func() {
				BeforeEach(func() {
					mockJwtCrypto.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{
						UAC:               uac,
						PostcodeValidated: true,
						UacInfo: busapi.UacInfo{
							InstrumentName: instrumentName,
							CaseID:         caseID,
						},
					}, nil)
				})

				It("Allows the context to continue", func() {
					Expect(httpRecorder.Code).To(Equal(http.StatusOK))
					body := httpRecorder.Body.Bytes()
					Expect(string(body)).To(Equal("true"))
				})
			})
		})

		Context("When a token cannot be decrypted", func() {
			BeforeEach(func() {
				mockJwtCrypto.On("DecryptJWT", mock.Anything).Return(nil, fmt.Errorf("Explosions"))
			})

			It("return unauthorized", func() {
				Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
				body := httpRecorder.Body.Bytes()
				Expect(strings.Contains(string(body), `<span class="btn__inner">Access survey</span>`)).To(BeTrue())
			})
		})
	})

	Context("When there is no token", func() {
		BeforeEach(func() {
			httpRouter.Use(auth.AuthenticatedWithUacAndPostcode)
			httpRouter.GET("/", func(context *gin.Context) {
				context.JSON(200, true)
			})
		})

		It("return unauthorized", func() {
			Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
			body := httpRecorder.Body.Bytes()
			Expect(strings.Contains(string(body), `<span class="btn__inner">Access survey</span>`)).To(BeTrue())
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

	Context("When someone has a session without a validated postcode", func() {
		BeforeEach(func() {
			mockJwtCrypto.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{
				PostcodeValidated: false,
				UacInfo: busapi.UacInfo{
					InstrumentName: instrumentName,
					CaseID:         caseID,
				},
			}, nil)
		})

		It("returns false and an empty claim", func() {
			Expect(httpRecorder.Code).To(Equal(http.StatusOK))
			body := httpRecorder.Body.Bytes()
			Expect(string(body)).To(Equal(`{"HasSession":false,"Claim":null}`))
		})
	})

	Context("When someone has a session with a validated postcode", func() {
		BeforeEach(func() {
			mockJwtCrypto.On("DecryptJWT", mock.Anything).Return(&authenticate.UACClaims{
				PostcodeValidated: true,
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
				`{"HasSession":true,"Claim":{"uac":"","postcode_validated":true,"instrument_name":"foobar","case_id":"fizzbuzz"}}`,
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

var _ = DescribeTable("ValidatePostcode",
	func(enteredPostcode, casePostcode string, expected bool) {
		auth := &authenticate.Auth{}
		Expect(auth.ValidatePostcode(enteredPostcode, casePostcode)).To(Equal(expected))
	},
	Entry("identical", "NP10 8XG", "NP10 8XG", true),
	Entry("spacing1", "NP10 8XG", "NP108XG", true),
	Entry("spacing2", "NP10  8XG", "NP10 8XG", true),
	Entry("spacing3", " NP10 8XG", "NP10  8XG ", true),
	Entry("casing", "Np10 8XG", "nP10 8Xg", true),
	Entry("casing and spacing", "Np10   8Xg", "  np108XG  ", true),
	Entry("no match", "Np10   8Xg", "  np18XG  ", false),
	Entry("no match 2", "NP10 8XG", "NP 8XG", false),
)
