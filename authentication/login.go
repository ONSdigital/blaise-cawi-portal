package login

import "net/http"

type Auth struct {
	JWTSecret string
	UacURL    string
	UacClient *http.Client
}
