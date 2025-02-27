package webserver

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/blaiserestapi"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/ONSdigital/blaise-cawi-portal/languagemanager"
	"github.com/ONSdigital/blaise-cawi-portal/utils"
	"github.com/blendle/zapdriver"
	"github.com/gin-contrib/secure"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-contrib/sessions/redis"
	"github.com/gin-gonic/gin"
	"github.com/kelseyhightower/envconfig"
	csrf "github.com/srbry/gin-csrf"
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

	defer func() {
		if err := logger.Sync(); err != nil {
			log.Println("Error occurred during logger synchronization:", err)
		}
	}()
	return logger, nil
}

func CSRFErrorFunc(csrfManager csrf.CSRFManager, config *Config, logger *zap.Logger, languageManger languagemanager.LanguageManagerInterface) func(*gin.Context) {
	return func(context *gin.Context) {
		logger.Info("CSRF mismatch", utils.GetRequestSource(context)...)
		var errorMessage string
		isWelsh := languageManger.IsWelsh(context)
		if isWelsh {
			errorMessage = "Cais wedi dod i ben, triwch eto"
		} else {
			errorMessage = "Request timed out, please try again"
		}
		context.HTML(http.StatusForbidden, "login.tmpl", gin.H{
			"uac16":      config.UacKind == "uac16",
			"info":       errorMessage,
			"csrf_token": csrfManager.GetToken(context),
			"welsh":      isWelsh,
		})
		context.Abort()
	}
}

func NewCSRFManager(config *Config, logger *zap.Logger, languageManger languagemanager.LanguageManagerInterface) csrf.CSRFManager {
	csrfManager := &csrf.DefaultCSRFManager{
		SessionName: "session",
		Secret:      config.SessionSecret,
	}

	csrfManager.ErrorFunc = CSRFErrorFunc(csrfManager, config, logger, languageManger)

	return csrfManager
}

func UserSessionStore(config *Config) (sessions.Store, error) {
	var (
		store sessions.Store
	)
	if config.DevMode {
		store = cookie.NewStore([]byte(config.SessionSecret), []byte(config.EncryptionSecret))
	} else {
		var err error
		store, err = redis.NewStore(10, "tcp", config.RedisSessionDB, "", []byte(config.SessionSecret), []byte(config.EncryptionSecret))
		if err != nil {
			return nil, err
		}
	}
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   60 * 60 * 24, // 1 days
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	return store, nil
}

func WrapWelsh(welsh bool) gin.H {
	return gin.H{
		"welsh": welsh,
	}
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

	store, err := UserSessionStore(server.Config)
	if err != nil {
		log.Fatalf("Could not connect to session database: %s", err)
	}

	cookieStore := cookie.NewStore([]byte(server.Config.SessionSecret), []byte(server.Config.EncryptionSecret))
	cookieStore.Options(sessions.Options{
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 30, // 30 days
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	languageStore := cookie.NewStore([]byte(server.Config.SessionSecret), []byte(server.Config.EncryptionSecret))
	languageStore.Options(sessions.Options{
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 365, // 365 days
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	sessionStores := []sessions.SessionStore{
		{
			Name:  "session",
			Store: cookieStore,
		},
		{
			Name:  "user_session",
			Store: store,
		},
		{
			Name:  "session_validation",
			Store: store,
		},
		{
			Name:  "language_session",
			Store: languageStore,
		},
	}
	httpRouter.Use(sessions.SessionsManyStores(sessionStores))

	//This router has access to all templates in the templates folder
	httpRouter.TrustedPlatform = gin.PlatformGoogleAppEngine
	httpRouter.SetFuncMap(template.FuncMap{
		"WrapWelsh": WrapWelsh,
	})
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

	languageManager := &languagemanager.Manager{SessionName: "language_session"}
	csrfManager := NewCSRFManager(server.Config, logger, languageManager)

	auth := &authenticate.Auth{
		JWTCrypto:     jwtCrypto,
		BlaiseRestApi: blaiseRestApi,
		Logger:        logger,
		BusApi: &busapi.BusApi{
			BaseUrl: server.Config.BusUrl,
			Client:  client,
		},
		UacKind:         server.Config.UacKind,
		CSRFManager:     csrfManager,
		LanguageManager: languageManager,
	}

	authController := &AuthController{
		Auth:            auth,
		Logger:          logger,
		UacKind:         server.Config.UacKind,
		CSRFManager:     csrfManager,
		LanguageManager: languageManager,
	}

	securityController := &SecurityController{}

	securityController.AddRoutes(httpRouter)

	authController.AddRoutes(httpRouter)
	instrumentController := &InstrumentController{
		Auth:            auth,
		JWTCrypto:       jwtCrypto,
		Logger:          logger,
		CatiUrl:         server.Config.CatiUrl,
		HttpClient:      httpClient,
		LanguageManager: languageManager,
	}
	instrumentController.AddRoutes(httpRouter)
	healthController := &HealthController{}
	healthController.AddRoutes(httpRouter)

	httpRouter.GET("/", authController.LoginEndpoint)

	httpRouter.Any("/language/:lang", func(context *gin.Context) {
		if languagemanager.GetLangFromParam(context) == "welsh" {
			languageManager.SetWelsh(context, true)
		} else {
			languageManager.SetWelsh(context, false)
		}
		context.Status(200)
	})

	httpRouter.NoRoute(func(context *gin.Context) {
		context.HTML(http.StatusOK, "not_found.tmpl", gin.H{"welsh": languageManager.IsWelsh(context)})
	})

	return httpRouter
}
