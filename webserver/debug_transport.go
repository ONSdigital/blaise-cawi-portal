package webserver

import (
	"net/http"

	"github.com/ONSdigital/blaise-cawi-portal/utils"
	"go.uber.org/zap"
)

type debugTransport struct {
	Logger *zap.Logger
}

func (debugTransport *debugTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	debugTransport.Logger.Debug("Proxy round trip debug",
		zap.String("Method", r.Method),
		zap.String("Host", utils.SanitiseLogInput(r.URL.Host)),
		zap.String("Path", utils.SanitiseLogInput(r.URL.Path)),
	)
	return http.DefaultTransport.RoundTrip(r)
}
