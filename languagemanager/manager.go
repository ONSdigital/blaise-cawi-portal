package languagemanager

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

//go:generate mockery
type LanguageManagerInterface interface {
	IsWelsh(*gin.Context) bool
	SetWelsh(*gin.Context, bool)
	LanguageError(map[string]string, *gin.Context) string
}

type Manager struct {
	SessionName string
	Logger      *zap.Logger
}

func (manager *Manager) logger() *zap.Logger {
	if manager.Logger != nil {
		return manager.Logger
	}
	return zap.L()
}

func (manager *Manager) IsWelsh(context *gin.Context) bool {
	session := sessions.DefaultMany(context, manager.SessionName)
	switch isWelsh := session.Get("welsh").(type) {
	case bool:
		return isWelsh
	}
	return false
}

func (manager *Manager) SetWelsh(context *gin.Context, welsh bool) {
	session := sessions.DefaultMany(context, manager.SessionName)
	session.Set("welsh", welsh)
	err := session.Save()
	if err != nil {
		manager.logger().Error("Failed to save language session", zap.Error(err))
	}
}

func (manager *Manager) LanguageError(err map[string]string, context *gin.Context) string {
	if manager.IsWelsh(context) {
		return err["welsh"]
	}
	return err["english"]
}
