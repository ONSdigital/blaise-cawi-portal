package webserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestDebugTransportRoundTripLogsRequestMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	loggerCore, logs := observer.New(zap.DebugLevel)
	transport := &debugTransport{Logger: zap.New(loggerCore)}

	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatalf("expected successful round trip, got error: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.StatusCode)
	}

	if logs.Len() != 1 {
		t.Fatalf("expected one debug log entry, got %d", logs.Len())
	}

	entry := logs.All()[0]
	if entry.Message != "Proxy round trip debug" {
		t.Fatalf("expected debug log message %q, got %q", "Proxy round trip debug", entry.Message)
	}

	if got, ok := entry.ContextMap()["Method"]; !ok || got != http.MethodGet {
		t.Fatalf("expected Method %q field in debug log context, got %v", http.MethodGet, got)
	}

	if _, ok := entry.ContextMap()["Host"]; !ok {
		t.Fatal("expected Host field in debug log context")
	}

	if got, ok := entry.ContextMap()["Path"]; !ok || got != "" {
		t.Fatalf("expected empty Path field in debug log context, got %v", got)
	}
}
