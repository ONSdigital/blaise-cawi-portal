package webserver

import (
	"net/http"

	"go.uber.org/zap"
)

type debugTransport struct {
	Logger *zap.Logger
}

func (debugTransport *debugTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	debugTransport.Logger.Debug("Proxy round trip debug",
		zap.String("Method", r.Method),
		zap.String("Host", r.URL.Host),
		zap.String("Path", r.URL.Path),
	)
	return http.DefaultTransport.RoundTrip(r)
}
