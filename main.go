package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ONSdigital/blaise-cawi-portal/webserver"
	"go.uber.org/zap"
)

const shutdownTimeout = 10 * time.Second

func main() {
	config, err := webserver.LoadConfig()
	if err != nil {
		zap.L().Fatal("Failed to load configuration", zap.Error(err))
	}

	server := &webserver.Server{Config: config}
	httpRouter, err := server.SetupRouter()
	if err != nil {
		zap.L().Fatal("Failed to set up router", zap.Error(err))
	}
	defer func() {
		_ = zap.L().Sync()
	}()

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", config.Port),
		Handler: httpRouter,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err = <-errCh:
		if err != nil {
			zap.L().Fatal("Failed to start HTTP server", zap.Error(err))
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		zap.L().Info("Shutting down HTTP server")
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			zap.L().Fatal("Failed to shut down HTTP server", zap.Error(err))
		}

		if err = <-errCh; err != nil {
			zap.L().Fatal("HTTP server exited with error during shutdown", zap.Error(err))
		}
	}
}
