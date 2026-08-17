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
	"go.uber.org/zap/zapcore"
)

const shutdownTimeout = 10 * time.Second

func main() {
	bootstrapLogger := newBootstrapLogger()
	defer func() {
		_ = bootstrapLogger.Sync()
	}()

	exitCode := runApp(run, bootstrapLogger)
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func runApp(startFn func() error, logger *zap.Logger) int {
	if err := startFn(); err != nil {
		logger.Error("Service startup failed", zap.Error(err))
		return 1
	}

	return 0
}

func bootstrapLoggerConfig() zap.Config {
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stderr"}
	config.ErrorOutputPaths = []string{"stderr"}

	return config
}

func newBootstrapLogger() *zap.Logger {
	logger, err := bootstrapLoggerConfig().Build()
	if err == nil {
		return logger
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.Lock(os.Stderr),
		zapcore.ErrorLevel,
	)
	return zap.New(core)
}

func run() error {
	config, err := webserver.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	server := &webserver.Server{Config: config}
	httpRouter, err := server.SetupRouter()
	if err != nil {
		return fmt.Errorf("failed to set up router: %w", err)
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
			return fmt.Errorf("failed to start HTTP server: %w", err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		zap.L().Info("Shutting down HTTP server")
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("failed to shut down HTTP server: %w", err)
		}

		if err = <-errCh; err != nil {
			return fmt.Errorf("HTTP server exited with error during shutdown: %w", err)
		}
	}

	return nil
}
