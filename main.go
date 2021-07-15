package main

import "github.com/ONSdigital/blaise-cawi-portal/webserver"

type Config struct {
	SessionSecret    string `split_words:"true"`
	EncryptionSecret string `split_words:"true"`
	CawiURL          string `split_words:"true"`
	JWTSecret        string `split_words:"true"`
	UacURL           string `split_words:"true"`
	UacClientID      string `split_words:"true"`
}

func main() {
	// httpClient := &http.Client{}
	// httpRouter := gin.Default()

	// var config Config
	// if err := envconfig.Process("", &config); err != nil {
	// 	log.Fatal(err.Error())
	// }

	// auth := &Auth{
	// 	JWTSecret: config.JWTSecret,
	// 	UacURL:    config.UacURL,
	// 	UacClient: client,
	// }
	server := &webserver.Server{}
	httpRouter := server.SetupRouter()
	httpRouter.Run(":8080")
}
