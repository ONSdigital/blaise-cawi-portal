package foo

import (
	"fmt"
	"log"

	"github.com/ONSdigital/blaise-cawi-portal/webserver"
)

func foo() {
	config, err := webserver.LoadConfig()
	if err != nil {
		log.Fatal(err.Error())
	}

	server := &webserver.Server{Config: config}
	httpRouter := server.SetupRouter()
	httpRouter.Run(fmt.Sprintf(":%s", config.Port))
}