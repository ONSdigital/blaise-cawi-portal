package main

import (
	"errors"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestBootstrapLoggerConfigUsesStderr(t *testing.T) {
	config := bootstrapLoggerConfig()

	if len(config.OutputPaths) != 1 || config.OutputPaths[0] != "stderr" {
		t.Fatalf("OutputPaths = %v, want [stderr]", config.OutputPaths)
	}

	if len(config.ErrorOutputPaths) != 1 || config.ErrorOutputPaths[0] != "stderr" {
		t.Fatalf("ErrorOutputPaths = %v, want [stderr]", config.ErrorOutputPaths)
	}
}

func TestRunAppLogsStartupErrorAndReturnsExitCodeOne(t *testing.T) {
	observedCore, observedLogs := observer.New(zap.ErrorLevel)
	logger := zap.New(observedCore)

	exitCode := runApp(func() error {
		return errors.New("startup failed")
	}, logger)

	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}

	entries := observedLogs.All()
	if len(entries) != 1 {
		t.Fatalf("log entry count = %d, want 1", len(entries))
	}

	entry := entries[0]
	if entry.Message != "Service startup failed" {
		t.Fatalf("log message = %q, want %q", entry.Message, "Service startup failed")
	}

	if got := entry.ContextMap()["error"]; got != "startup failed" {
		t.Fatalf("error field = %v, want %q", got, "startup failed")
	}
}

func TestRunAppReturnsZeroOnSuccess(t *testing.T) {
	observedCore, observedLogs := observer.New(zap.ErrorLevel)
	logger := zap.New(observedCore)

	exitCode := runApp(func() error {
		return nil
	}, logger)

	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}

	if observedLogs.Len() != 0 {
		t.Fatalf("expected no error logs, got %d", observedLogs.Len())
	}
}
