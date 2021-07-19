package webserver

import (
	"context"
	"fmt"
	"os"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/idtoken"
)

type Server struct{}

func (server *Server) SetupRouter() *gin.Engine {
	httpRouter := gin.Default()

	store := cookie.NewStore([]byte(os.Getenv("SESSION_SECRET")), []byte(os.Getenv("ENCRYPTION_SECRET")))
	httpRouter.Use(sessions.Sessions("session", store))
	//This router has access to all templates in the templates folder
	httpRouter.AppEngine = true
	httpRouter.LoadHTMLGlob("templates/*")

	client, err := idtoken.NewClient(context.Background(), os.Getenv("BUS_CLIENT_ID"))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	auth := &authenticate.Auth{
		JWTSecret: os.Getenv("JWT_SECRET"),
		BusApi: &busapi.BusApi{
			BaseUrl: os.Getenv("BUS_URL"),
			Client:  client,
		},
	}

	authController := &AuthController{
		Auth: auth,
	}
	authController.AddRoutes(httpRouter)
	instrumentController := &InstrumentController{
		Auth: auth,
	}
	instrumentController.AddRoutes(httpRouter)
	healthController := &HealthController{}
	healthController.AddRoutes(httpRouter)
	return httpRouter
}
