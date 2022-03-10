package languagemanager

import (
	"fmt"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

//Generate mocks by running "go generate ./..."
//go:generate mockery --name LanguageManagerInterface
type LanguageManagerInterface interface {
	IsWelsh(*gin.Context) bool
	SetWelsh(*gin.Context, bool)
	LanguageError(map[string]string, *gin.Context) string
}

type Manager struct {
	SessionName string
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
		fmt.Printf("Error saving session: %s\n", err)
	}
}

func (manager *Manager) LanguageError(err map[string]string, context *gin.Context) string {
	if manager.IsWelsh(context) {
		return err["welsh"]
	}
	return err["english"]
}
