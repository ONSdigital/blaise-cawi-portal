package languagemanager_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ONSdigital/blaise-cawi-portal/languagemanager"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func TestIsWelsh(t *testing.T) {
	manager := &languagemanager.Manager{SessionName: "language_session"}

	runCase := func(t *testing.T, value interface{}, setValue bool, expected bool) {
		t.Helper()

		recorder := httptest.NewRecorder()
		router := gin.Default()
		store := cookie.NewStore([]byte("secret"))
		router.Use(sessions.SessionsMany([]string{"language_session"}, store))

		router.GET("/", func(c *gin.Context) {
			if setValue {
				session := sessions.DefaultMany(c, "language_session")
				session.Set("welsh", value)
				if err := session.Save(); err != nil {
					t.Fatalf("session.Save() error: %v", err)
				}
			}
			if got := manager.IsWelsh(c); got != expected {
				t.Fatalf("IsWelsh() = %v, want %v", got, expected)
			}
			c.Status(http.StatusOK)
		})

		req, err := http.NewRequest(http.MethodGet, "/", nil)
		if err != nil {
			t.Fatalf("http.NewRequest() error: %v", err)
		}
		router.ServeHTTP(recorder, req)
	}

	t.Run("returns true when session welsh is true", func(t *testing.T) {
		runCase(t, true, true, true)
	})
	t.Run("returns false when session welsh is false", func(t *testing.T) {
		runCase(t, false, true, false)
	})
	t.Run("returns false when session welsh is unset", func(t *testing.T) {
		runCase(t, nil, false, false)
	})
	t.Run("returns false when session welsh is not bool", func(t *testing.T) {
		runCase(t, "foo", true, false)
	})
}

func TestSetWelsh(t *testing.T) {
	manager := &languagemanager.Manager{SessionName: "language_session"}
	recorder := httptest.NewRecorder()
	router := gin.Default()
	store := cookie.NewStore([]byte("secret"))
	router.Use(sessions.SessionsMany([]string{"language_session"}, store))

	router.GET("/set", func(c *gin.Context) {
		manager.SetWelsh(c, true)
		c.Status(http.StatusNoContent)
	})

	router.GET("/check", func(c *gin.Context) {
		if manager.IsWelsh(c) {
			c.Status(http.StatusOK)
			return
		}
		c.Status(http.StatusConflict)
	})

	setReq, err := http.NewRequest(http.MethodGet, "/set", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error: %v", err)
	}
	router.ServeHTTP(recorder, setReq)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}

	cookies := recorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected cookies to be set")
	}

	checkRecorder := httptest.NewRecorder()
	checkReq, err := http.NewRequest(http.MethodGet, "/check", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error: %v", err)
	}
	checkReq.AddCookie(cookies[0])
	router.ServeHTTP(checkRecorder, checkReq)

	if checkRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", checkRecorder.Code, http.StatusOK)
	}
}

func TestLanguageError(t *testing.T) {
	manager := &languagemanager.Manager{SessionName: "language_session"}
	languageErrors := map[string]string{"english": "Please continue in English", "welsh": "Parhewch yn Gymraeg"}

	t.Run("returns welsh error when session is welsh", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		router := gin.Default()
		store := cookie.NewStore([]byte("secret"))
		router.Use(sessions.SessionsMany([]string{"language_session"}, store))

		router.GET("/", func(c *gin.Context) {
			session := sessions.DefaultMany(c, "language_session")
			session.Set("welsh", true)
			if err := session.Save(); err != nil {
				t.Fatalf("session.Save() error: %v", err)
			}
			if got := manager.LanguageError(languageErrors, c); got != languageErrors["welsh"] {
				t.Fatalf("LanguageError() = %q, want %q", got, languageErrors["welsh"])
			}
			c.Status(http.StatusOK)
		})

		req, err := http.NewRequest(http.MethodGet, "/", nil)
		if err != nil {
			t.Fatalf("http.NewRequest() error: %v", err)
		}
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
	})

	t.Run("returns english error when session is not welsh", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		router := gin.Default()
		store := cookie.NewStore([]byte("secret"))
		router.Use(sessions.SessionsMany([]string{"language_session"}, store))

		router.GET("/", func(c *gin.Context) {
			if got := manager.LanguageError(languageErrors, c); got != languageErrors["english"] {
				t.Fatalf("LanguageError() = %q, want %q", got, languageErrors["english"])
			}
			c.Status(http.StatusOK)
		})

		req, err := http.NewRequest(http.MethodGet, "/", nil)
		if err != nil {
			t.Fatalf("http.NewRequest() error: %v", err)
		}
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
	})
}
