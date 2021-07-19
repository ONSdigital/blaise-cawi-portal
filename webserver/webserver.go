package webserver

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/idtoken"
)

type Config struct {
	SessionSecret    string `split_words:"true"`
	EncryptionSecret string `split_words:"true"`
	CatiUrl          string `split_words:"true"`
	JWTSecret        string `split_words:"true"`
	BusUrl           string `split_words:"true"`
	BusClientId      string `split_words:"true"`
	Port             string
}

type Server struct {
	Config *Config
}

func (server *Server) SetupRouter() *gin.Engine {
	httpRouter := gin.Default()
	httpClient := &http.Client{}

	store := cookie.NewStore([]byte(server.Config.SessionSecret), []byte(server.Config.EncryptionSecret))
	httpRouter.Use(sessions.Sessions("session", store))
	//This router has access to all templates in the templates folder
	httpRouter.AppEngine = true
	httpRouter.LoadHTMLGlob("templates/*")

	client, err := idtoken.NewClient(context.Background(), server.Config.BusClientId)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	auth := &authenticate.Auth{
		JWTSecret: server.Config.JWTSecret,
		BusApi: &busapi.BusApi{
			BaseUrl: server.Config.BusUrl,
			Client:  client,
		},
	}

	authController := &AuthController{
		Auth: auth,
	}
	authController.AddRoutes(httpRouter)
	instrumentController := &InstrumentController{
		Auth:       auth,
		CatiUrl:    server.Config.CatiUrl,
		HttpClient: httpClient,
	}
	instrumentController.AddRoutes(httpRouter)
	healthController := &HealthController{}
	healthController.AddRoutes(httpRouter)
	return httpRouter
}
