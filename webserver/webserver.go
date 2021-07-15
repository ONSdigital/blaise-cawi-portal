package webserver

import (
	"github.com/gin-gonic/gin"
)

type Server struct {
}

func (server *Server) SetupRouter() *gin.Engine {
	httpRouter := gin.Default()
	//This router has access to all templates in the templates folder
	httpRouter.AppEngine = true
	httpRouter.LoadHTMLGlob("templates/*")

	authController := &AuthController{}
	authController.AddRoutes(httpRouter)
	return httpRouter
}
