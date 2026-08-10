package csrf

import (
	"crypto/sha1"
	"encoding/base64"
	"io"
	"net/http"

	"github.com/dchest/uniuri"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	csrfSecret = "csrfSecret"
	csrfSalt   = "csrfSalt"
	csrfToken  = "csrfToken"
)

var defaultIgnoreMethods = []string{"GET", "HEAD", "OPTIONS"}

var defaultErrorFunc = func(c *gin.Context) {
	c.AbortWithStatus(http.StatusForbidden)
}

var defaultTokenGetter = func(c *gin.Context) string {
	r := c.Request

	if t := r.FormValue("_csrf"); len(t) > 0 {
		return t
	} else if t := r.URL.Query().Get("_csrf"); len(t) > 0 {
		return t
	} else if t := r.Header.Get("X-CSRF-TOKEN"); len(t) > 0 {
		return t
	} else if t := r.Header.Get("X-XSRF-TOKEN"); len(t) > 0 {
		return t
	}

	return ""
}

// CSRFManager methods for adding a gin middleware and token validation.
type CSRFManager interface {
	Middleware() gin.HandlerFunc
	GetToken(*gin.Context) string
}

// DefaultCSRFManager stores configurations for a CSRF middleware.
type DefaultCSRFManager struct {
	Secret        string
	IgnoreMethods []string
	ErrorFunc     gin.HandlerFunc
	TokenGetter   func(c *gin.Context) string
	SessionName   string
}

// Middleware validates CSRF token.
func (csrfManager *DefaultCSRFManager) Middleware() gin.HandlerFunc {
	ignoreMethods := csrfManager.IgnoreMethods
	errorFunc := csrfManager.ErrorFunc
	tokenGetter := csrfManager.TokenGetter

	if ignoreMethods == nil {
		ignoreMethods = defaultIgnoreMethods
	}

	if errorFunc == nil {
		errorFunc = defaultErrorFunc
	}

	if tokenGetter == nil {
		tokenGetter = defaultTokenGetter
	}

	return func(c *gin.Context) {
		session := csrfManager.getSession(c)
		c.Set(csrfSecret, csrfManager.Secret)

		if inArray(ignoreMethods, c.Request.Method) {
			c.Next()
			return
		}

		salt, ok := session.Get(csrfSalt).(string)

		if !ok || len(salt) == 0 {
			errorFunc(c)
			return
		}

		token := tokenGetter(c)

		if tokenize(csrfManager.Secret, salt) != token {
			errorFunc(c)
			return
		}

		c.Next()
	}
}

// GetToken returns a CSRF token.
func (csrfManager *DefaultCSRFManager) GetToken(c *gin.Context) string {
	session := csrfManager.getSession(c)

	if t, ok := c.Get(csrfToken); ok {
		return t.(string)
	}

	salt, ok := session.Get(csrfSalt).(string)
	if !ok {
		salt = uniuri.New()
		session.Set(csrfSalt, salt)
		if err := session.Save(); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return ""
		}
	}
	token := tokenize(csrfManager.Secret, salt)
	c.Set(csrfToken, token)

	return token
}

func (csrfManager *DefaultCSRFManager) getSession(c *gin.Context) sessions.Session {
	var session sessions.Session
	if csrfManager.SessionName == "" {
		session = sessions.Default(c)
	} else {
		session = sessions.DefaultMany(c, csrfManager.SessionName)
	}
	return session
}

func tokenize(secret, salt string) string {
	h := sha1.New()
	_, _ = io.WriteString(h, salt+"-"+secret)
	hash := base64.URLEncoding.EncodeToString(h.Sum(nil))

	return hash
}

func inArray(arr []string, value string) bool {
	for _, v := range arr {
		if v == value {
			return true
		}
	}

	return false
}
