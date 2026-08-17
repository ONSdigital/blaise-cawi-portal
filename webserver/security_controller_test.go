package webserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ONSdigital/blaise-cawi-portal/webserver"

	"github.com/gin-gonic/gin"
)

func TestSecurityControllerTraceIsBlocked(t *testing.T) {
	router := gin.Default()
	securityController := &webserver.SecurityController{}
	securityController.AddRoutes(router)

	tests := []struct {
		name string
		path string
	}{
		{name: "root", path: "/"},
		{name: "short path", path: "/foo"},
		{name: "longer path", path: "/foo/bar"},
		{name: "long path", path: "/foo/bar/baz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodTrace, tt.path, nil)
			if err != nil {
				t.Fatalf("http.NewRequest() error: %v", err)
			}
			router.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}
