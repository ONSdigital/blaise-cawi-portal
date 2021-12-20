package webserver

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/blaiserestapi"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/blendle/zapdriver"
	"github.com/gin-contrib/secure"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/redis"
	"github.com/gin-gonic/gin"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/api/idtoken"
)

const CDN = "https://cdn.ons.gov.uk"

var (
	srcHosts              = fmt.Sprintf("'self' %s", CDN)
	defaultSRC            = fmt.Sprintf("default-src %s 'unsafe-inline'", srcHosts)
	fontSRC               = fmt.Sprintf("font-src %s data:", srcHosts)
	imgSRC                = fmt.Sprintf("img-src %s data:", srcHosts)
	contentSecurityPolicy = fmt.Sprintf("%s; %s; %s", defaultSRC, fontSRC, imgSRC)
)

type Config struct {
	RedisSessionDB   string `default:"localhost:6379" split_words:"true"`
	SessionSecret    string `required:"true" split_words:"true"`
	EncryptionSecret string `required:"true" split_words:"true"`
	CatiUrl          string `required:"true" split_words:"true"`
	JWTSecret        string `required:"true" split_words:"true"`
	BusUrl           string `required:"true" split_words:"true"`
	BusClientId      string `required:"true" split_words:"true"`
	BlaiseRestApi    string `required:"true" split_words:"true"`
	Serverpark       string `default:"gusty"`
	Port             string `default:"8080"`
	UacKind          string `default:"uac" split_words:"true"`
	DevMode          bool   `default:"false" split_words:"true"`
	Debug            bool   `default:"false"`
}

func LoadConfig() (*Config, error) {
	var config Config
	if err := envconfig.Process("", &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func NewLogger(config *Config) (*zap.Logger, error) {
	var (
		logger *zap.Logger
		err    error
	)
	if config.DevMode {
		// logger, err = zap.NewDevelopment()
		logger, err = zapdriver.NewProduction()
	} else {
		var zapOptions []zap.Option
		if config.Debug {
			zapOptions = append(zapOptions,
				zap.IncreaseLevel(zap.LevelEnablerFunc(func(level zapcore.Level) bool {
					return true
				})),
			)
		}
		logger, err = zapdriver.NewProduction(zapOptions...)
	}
	if err != nil {
		return nil, err
	}
	defer logger.Sync()
	return logger, nil
}

type Server struct {
	Config *Config
}

func (server *Server) SetupRouter() *gin.Engine {
	logger, err := NewLogger(server.Config)
	if err != nil {
		log.Fatalf("Error setting up logger: %s", err)
	}
	httpRouter := gin.Default()
	httpClient := &http.Client{}

	securityConfig := secure.DefaultConfig()
	securityConfig.ContentSecurityPolicy = contentSecurityPolicy

	if server.Config.DevMode {
		securityConfig.IsDevelopment = true
	}

	httpRouter.Use(secure.New(securityConfig))

	store, err := redis.NewStore(10, "tcp", server.Config.RedisSessionDB, "", []byte(server.Config.SessionSecret), []byte(server.Config.EncryptionSecret))
	if err != nil {
		log.Fatalf("Could not connect to session database: %s", err)
	}
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 30, // 30 days
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	httpRouter.Use(sessions.Sessions("session", store))

	//This router has access to all templates in the templates folder
	httpRouter.AppEngine = true
	httpRouter.LoadHTMLGlob("templates/*")
	httpRouter.Static("/assets", "./assets")

	client, err := idtoken.NewClient(context.Background(), server.Config.BusClientId)
	if err != nil {
		logger.Fatal("Error creating bus client", zap.Error(err))
	}

	jwtCrypto := &authenticate.JWTCrypto{
		JWTSecret: server.Config.JWTSecret,
	}

	blaiseRestApi := &blaiserestapi.BlaiseRestApi{
		BaseUrl:    server.Config.BlaiseRestApi,
		Serverpark: server.Config.Serverpark,
		Client:     &http.Client{},
	}

	auth := &authenticate.Auth{
		JWTCrypto:     jwtCrypto,
		BlaiseRestApi: blaiseRestApi,
		Logger:        logger,
		BusApi: &busapi.BusApi{
			BaseUrl: server.Config.BusUrl,
			Client:  client,
		},
		CSRFSecret: server.Config.SessionSecret,
		UacKind:    server.Config.UacKind,
	}

	authController := &AuthController{
		Auth:       auth,
		Logger:     logger,
		CSRFSecret: server.Config.SessionSecret,
		UacKind:    server.Config.UacKind,
	}

	securityController := &SecurityController{}

	securityController.AddRoutes(httpRouter)

	authController.AddRoutes(httpRouter)
	instrumentController := &InstrumentController{
		Auth:       auth,
		JWTCrypto:  jwtCrypto,
		Logger:     logger,
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
