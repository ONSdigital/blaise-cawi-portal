package webserver

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/blaiserestapi"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/ONSdigital/blaise-cawi-portal/csrf"
	"github.com/ONSdigital/blaise-cawi-portal/languagemanager"
	"github.com/ONSdigital/blaise-cawi-portal/sessionkeys"
	"github.com/ONSdigital/blaise-cawi-portal/utils"
	"github.com/blendle/zapdriver"
	"github.com/gin-contrib/secure"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-contrib/sessions/redis"
	"github.com/gin-gonic/gin"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/api/idtoken"
)

const CDN = "https://cdn.ons.gov.uk"

const httpClientTimeout = 3 * time.Minute

const redisPoolSize = 10

var (
	srcHosts              = fmt.Sprintf("'self' %s", CDN)
	defaultSrc            = fmt.Sprintf("default-src %s 'unsafe-inline'", srcHosts)
	fontSrc               = fmt.Sprintf("font-src %s data:", srcHosts)
	imgSrc                = fmt.Sprintf("img-src %s data:", srcHosts)
	contentSecurityPolicy = fmt.Sprintf("%s; %s; %s", defaultSrc, fontSrc, imgSrc)
	idTokenNewClient      = idtoken.NewClient
)

type Config struct {
	RedisSessionDB   string `default:"localhost:6379" split_words:"true"`
	SessionSecret    string `required:"true" split_words:"true"`
	EncryptionSecret string `required:"true" split_words:"true"`
	CatiURL          string `required:"true" split_words:"true"`
	JWTSecret        string `required:"true" split_words:"true"`
	BusURL           string `required:"true" split_words:"true"`
	BusClientID      string `required:"true" split_words:"true"`
	BlaiseRestAPI    string `required:"true" split_words:"true"`
	Serverpark       string `default:"gusty"`
	Port             string `default:"8082"`
	UACKind          string `default:"uac" split_words:"true"`
	// DevMode switches the session backend to cookie store (no Redis) and relaxes security middleware; it does not affect log verbosity.
	DevMode bool `default:"false" split_words:"true"`
	Debug   bool `default:"false"`
}

func (config *Config) Validate() error {
	requiredFields := []struct {
		name  string
		value string
	}{
		{name: "SESSION_SECRET", value: config.SessionSecret},
		{name: "ENCRYPTION_SECRET", value: config.EncryptionSecret},
		{name: "CATI_URL", value: config.CatiURL},
		{name: "JWT_SECRET", value: config.JWTSecret},
		{name: "BUS_URL", value: config.BusURL},
		{name: "BUS_CLIENT_ID", value: config.BusClientID},
		{name: "BLAISE_REST_API", value: config.BlaiseRestAPI},
	}

	var missing []string
	for _, field := range requiredFields {
		if strings.TrimSpace(field.value) == "" {
			missing = append(missing, field.name)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("required config values are missing or empty: %s", strings.Join(missing, ", "))
	}

	return nil
}

func LoadConfig() (*Config, error) {
	var config Config
	if err := envconfig.Process("", &config); err != nil {
		return nil, err
	}
	if err := config.Validate(); err != nil {
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
		// DevMode does not enable debug logging; use DEBUG=true for that.
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
	zap.ReplaceGlobals(logger)
	return logger, nil
}

func CSRFErrorFunc(csrfManager csrf.CSRFManager, config *Config, logger *zap.Logger, languageManager languagemanager.LanguageManagerInterface) func(*gin.Context) {
	return func(context *gin.Context) {
		logger.Info("CSRF mismatch", utils.GetRequestSource(context)...)
		context.HTML(http.StatusForbidden, "login.tmpl", gin.H{
			"uac16":      config.UACKind == "uac16",
			"info":       languageManager.LanguageError(authenticate.CSRF_ERR, context),
			"csrf_token": csrfManager.GetToken(context),
			"welsh":      languageManager.IsWelsh(context),
		})
		context.Abort()
	}
}

func NewCSRFManager(config *Config, logger *zap.Logger, languageManager languagemanager.LanguageManagerInterface) csrf.CSRFManager {
	csrfManager := &csrf.DefaultCSRFManager{
		SessionName: sessionkeys.SessionName,
		Secret:      config.SessionSecret,
	}

	csrfManager.ErrorFunc = CSRFErrorFunc(csrfManager, config, logger, languageManager)

	return csrfManager
}

func UserSessionStore(config *Config) (sessions.Store, error) {
	var store sessions.Store
	if config.DevMode {
		store = cookie.NewStore([]byte(config.SessionSecret), []byte(config.EncryptionSecret))
	} else {
		var err error
		store, err = redis.NewStore(redisPoolSize, "tcp", config.RedisSessionDB, "", "", []byte(config.SessionSecret), []byte(config.EncryptionSecret))
		if err != nil {
			return nil, err
		}
	}
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   60 * 60 * 24, // 1 day
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

type routerControllers struct {
	authController       *AuthController
	instrumentController *InstrumentController
	securityController   *SecurityController
	healthController     *HealthController
	languageManager      languagemanager.LanguageManagerInterface
}

func (server *Server) configureSecurityMiddleware(httpRouter *gin.Engine) {
	securityConfig := secure.DefaultConfig()
	securityConfig.ContentSecurityPolicy = contentSecurityPolicy

	if server.Config.DevMode {
		securityConfig.IsDevelopment = true
	}

	httpRouter.Use(secure.New(securityConfig))
}

func newCookieSessionStore(config *Config, maxAgeSeconds int) sessions.Store {
	store := cookie.NewStore([]byte(config.SessionSecret), []byte(config.EncryptionSecret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   maxAgeSeconds,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	return store
}

func (server *Server) configureSessionMiddleware(httpRouter *gin.Engine) error {
	store, err := UserSessionStore(server.Config)
	if err != nil {
		return err
	}

	cookieStore := newCookieSessionStore(server.Config, 60*60*24*30)
	languageStore := newCookieSessionStore(server.Config, 60*60*24*365)

	sessionStores := []sessions.SessionStore{
		{
			Name:  sessionkeys.SessionName,
			Store: cookieStore,
		},
		{
			Name:  sessionkeys.UserSessionName,
			Store: store,
		},
		{
			Name:  sessionkeys.SessionValidationName,
			Store: store,
		},
		{
			Name:  sessionkeys.LanguageSessionName,
			Store: languageStore,
		},
	}

	httpRouter.Use(sessions.SessionsManyStores(sessionStores))

	return nil
}

func configureTemplateAndStaticContent(httpRouter *gin.Engine) {
	httpRouter.TrustedPlatform = gin.PlatformGoogleAppEngine
	httpRouter.SetFuncMap(template.FuncMap{
		"WrapWelsh": WrapWelsh,
	})
	httpRouter.LoadHTMLGlob("templates/*")
	httpRouter.Static("/assets", "./assets")
}

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: httpClientTimeout}
}

func (server *Server) buildControllers(logger *zap.Logger) (*routerControllers, error) {
	client, err := idTokenNewClient(context.Background(), server.Config.BusClientID)
	if err != nil {
		return nil, err
	}

	jwtCrypto := &authenticate.JWTCrypto{
		JWTSecret: server.Config.JWTSecret,
	}

	blaiseRestApi := &blaiserestapi.BlaiseRestAPI{
		BaseURL:    server.Config.BlaiseRestAPI,
		Serverpark: server.Config.Serverpark,
		Client:     newHTTPClient(),
		Logger:     logger,
	}

	languageManager := &languagemanager.Manager{SessionName: sessionkeys.LanguageSessionName, Logger: logger}
	csrfManager := NewCSRFManager(server.Config, logger, languageManager)

	auth := &authenticate.Auth{
		JWTCrypto:     jwtCrypto,
		BlaiseRestAPI: blaiseRestApi,
		Logger:        logger,
		BUSAPI: &busapi.BUSAPI{
			BaseURL: server.Config.BusURL,
			Client:  client,
		},
		UACKind:         server.Config.UACKind,
		CSRFManager:     csrfManager,
		LanguageManager: languageManager,
	}

	authController := &AuthController{
		Auth:            auth,
		CSRFManager:     csrfManager,
		LanguageManager: languageManager,
	}

	instrumentController := &InstrumentController{
		Auth:            auth,
		JWTCrypto:       jwtCrypto,
		Logger:          logger,
		CatiURL:         server.Config.CatiURL,
		HttpClient:      newHTTPClient(),
		Debug:           server.Config.Debug,
		LanguageManager: languageManager,
	}

	return &routerControllers{
		authController:       authController,
		instrumentController: instrumentController,
		securityController:   &SecurityController{},
		healthController:     &HealthController{},
		languageManager:      languageManager,
	}, nil
}

func registerControllerRoutes(httpRouter *gin.Engine, controllers *routerControllers) {
	controllers.securityController.AddRoutes(httpRouter)
	controllers.authController.AddRoutes(httpRouter)
	controllers.instrumentController.AddRoutes(httpRouter)
	controllers.healthController.AddRoutes(httpRouter)
}

func registerUtilityRoutes(httpRouter *gin.Engine, authController *AuthController, languageManager languagemanager.LanguageManagerInterface) {
	httpRouter.GET("/", authController.LoginEndpoint)

	setLanguage := func(context *gin.Context) {
		if strings.ToLower(context.Param("lang")) == "welsh" {
			languageManager.SetWelsh(context, true)
		} else {
			languageManager.SetWelsh(context, false)
		}
	}

	redirectToReferrerOrRoot := func(context *gin.Context) {
		referer := context.Request.Referer()
		if referer == "" {
			context.Redirect(http.StatusSeeOther, "/")
			return
		}

		refererURL, err := url.Parse(referer)
		if err != nil || refererURL.Path == "" {
			context.Redirect(http.StatusSeeOther, "/")
			return
		}

		target := refererURL.Path
		if refererURL.RawQuery != "" {
			target = fmt.Sprintf("%s?%s", target, refererURL.RawQuery)
		}

		context.Redirect(http.StatusSeeOther, target)
	}

	httpRouter.POST("/language/:lang", func(context *gin.Context) {
		setLanguage(context)
		context.Status(http.StatusOK)
	})

	httpRouter.GET("/language/:lang", func(context *gin.Context) {
		setLanguage(context)
		redirectToReferrerOrRoot(context)
	})

	httpRouter.NoRoute(func(context *gin.Context) {
		context.HTML(http.StatusNotFound, "not_found.tmpl", gin.H{"welsh": languageManager.IsWelsh(context)})
	})
}

func (server *Server) SetupRouter() (*gin.Engine, error) {
	logger, err := NewLogger(server.Config)
	if err != nil {
		return nil, fmt.Errorf("error setting up logger: %w", err)
	}

	httpRouter := gin.Default()
	server.configureSecurityMiddleware(httpRouter)

	if err := server.configureSessionMiddleware(httpRouter); err != nil {
		return nil, fmt.Errorf("could not connect to session database: %w", err)
	}

	configureTemplateAndStaticContent(httpRouter)

	controllers, err := server.buildControllers(logger)
	if err != nil {
		return nil, fmt.Errorf("error creating bus client: %w", err)
	}

	registerControllerRoutes(httpRouter, controllers)
	registerUtilityRoutes(httpRouter, controllers.authController, controllers.languageManager)

	return httpRouter, nil
}
