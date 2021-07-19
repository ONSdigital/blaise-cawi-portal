package main

import (
	"fmt"
	"log"

	"github.com/ONSdigital/blaise-cawi-portal/webserver"
	"github.com/kelseyhightower/envconfig"
)

func main() {
	config := &webserver.Config{}
	if err := envconfig.Process("", config); err != nil {
		log.Fatal(err.Error())
	}

	server := &webserver.Server{Config: config}
	httpRouter := server.SetupRouter()
	httpRouter.Run(fmt.Sprintf(":%s", config.Port))
}
