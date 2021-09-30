package webserver

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/gin-contrib/secure"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
	"github.com/kelseyhightower/envconfig"
	"google.golang.org/api/idtoken"
)

const CDN = "https://cdn.ons.gov.uk"

var (
	srcHosts              = fmt.Sprintf("'self' %s", CDN)
	defaultSRC            = fmt.Sprintf("default-src %s 'unsafe-inline'", srcHosts)
	fontSRC               = fmt.Sprintf("font-src %s data:", srcHosts)
	imgSRC                = fmt.Sprintf("img-src %s data:", srcHosts)
	contentSecurityPolicy = fmt.Sprintf("%s; %s; %s", defaultSRC, fontSRC, imgSRC)
	csrfMiddleware        func(http.Handler) http.Handler
)

type Config struct {
	SessionSecret    string `required:"true" split_words:"true"`
	EncryptionSecret string `required:"true" split_words:"true"`
	CatiUrl          string `required:"true" split_words:"true"`
	JWTSecret        string `required:"true" split_words:"true"`
	BusUrl           string `required:"true" split_words:"true"`
	BusClientId      string `required:"true" split_words:"true"`
	Serverpark       string `default:"gusty"`
	Port             string `default:"8080"`
	DevMode          bool   `default:"false" split_words:"true"`
}

func LoadConfig() (*Config, error) {
	var config Config
	if err := envconfig.Process("", &config); err != nil {
		return nil, err
	}
	return &config, nil
}

type Server struct {
	Config *Config
}

func (server *Server) SetupRouter() *gin.Engine {
	httpRouter := gin.Default()
	httpClient := &http.Client{}

	securityConfig := secure.DefaultConfig()
	securityConfig.ContentSecurityPolicy = contentSecurityPolicy
	httpRouter.Use(secure.New(securityConfig))

	if server.Config.DevMode {
		securityConfig.IsDevelopment = true
	}

	store := cookie.NewStore([]byte(server.Config.SessionSecret), []byte(server.Config.EncryptionSecret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   2 * 60 * 60,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	csrfMiddleware = csrf.Protect([]byte(server.Config.SessionSecret))

	httpRouter.Use(sessions.Sessions("session", store))

	//This router has access to all templates in the templates folder
	httpRouter.AppEngine = true
	httpRouter.LoadHTMLGlob("templates/*")

	client, err := idtoken.NewClient(context.Background(), server.Config.BusClientId)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	jwtCrypto := &authenticate.JWTCrypto{
		JWTSecret: server.Config.JWTSecret,
	}

	auth := &authenticate.Auth{
		JWTCrypto: jwtCrypto,
		BusApi: &busapi.BusApi{
			BaseUrl: server.Config.BusUrl,
			Client:  client,
		},
	}

	authController := &AuthController{
		Auth: auth,
	}

	securityController := &SecurityController{}

	securityController.AddRoutes(httpRouter)

	authController.AddRoutes(httpRouter)
	instrumentController := &InstrumentController{
		Auth:       auth,
		JWTCrypto:  jwtCrypto,
		CatiUrl:    server.Config.CatiUrl,
		HttpClient: httpClient,
	}
	instrumentController.AddRoutes(httpRouter)
	healthController := &HealthController{}
	healthController.AddRoutes(httpRouter)

	httpRouter.GET("/", authController.LoginEndpoint)

	httpRouter.NoRoute(func(context *gin.Context) {
		context.HTML(http.StatusOK, "not_found.tmpl", gin.H{})
	})

	return httpRouter
}
