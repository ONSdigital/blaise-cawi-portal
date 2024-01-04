package authenticate

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/blaiserestapi"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/ONSdigital/blaise-cawi-portal/languagemanager"
	"github.com/ONSdigital/blaise-cawi-portal/utils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	csrf "github.com/srbry/gin-csrf"
	"go.uber.org/zap"
)

const (
	SESSION_TIMEOUT_KEY = "session_timeout"
	JWT_TOKEN_KEY       = "jwt_token"
	SESSION_VALID_KEY   = "session_valid"
	ISSUER              = "social-surveys-web-portal"
)

var (
	INVALID_LENGTH_ERR = map[string]string{
		"english": "Enter your %s access code",
		"welsh":   "Rhowch eich cod mynediad sy'n cynnwys %s",
	}
	NOT_RECOGNISED_ERR = map[string]string{
		"english": "Access code not recognised. Enter the code again",
		"welsh":   "Nid yw'r cod mynediad yn cael ei gydnabod. Rhowch y cod eto",
	}
	INTERNAL_SERVER_ERR = map[string]string{
		"english": "We were unable to process your request, please try again",
		"welsh":   "Ni allwn brosesu eich cais, rhowch gynnig arall arni",
	}
)

//Generate mocks by running "go generate ./..."
//go:generate mockery --name AuthInterface
type AuthInterface interface {
	AuthenticatedWithUac(*gin.Context)
	Login(*gin.Context, sessions.Session)
	Logout(*gin.Context, sessions.Session)
	HasSession(*gin.Context) (bool, *UACClaims)
	NotAuthWithError(*gin.Context, string)
	RefreshToken(*gin.Context, sessions.Session, *UACClaims)
}

type Auth struct {
	BusApi          busapi.BusApiInterface
	JWTCrypto       JWTCryptoInterface
	BlaiseRestApi   blaiserestapi.BlaiseRestApiInterface
	Logger          *zap.Logger
	UacKind         string
	CSRFManager     csrf.CSRFManager
	LanguageManager languagemanager.LanguageManagerInterface
}

func (auth *Auth) AuthenticatedWithUac(context *gin.Context) {
	session := sessions.DefaultMany(context, "user_session")
	jwtToken := session.Get(JWT_TOKEN_KEY)

	if jwtToken == nil || !auth.SessionValid(context) {
		auth.notAuth(context)
		return
	}

	_, err := auth.JWTCrypto.DecryptJWT(jwtToken)
	if err != nil {
		log.Println(err)
		auth.notAuth(context)
		return
	}
	context.Next()
}

func (auth *Auth) HasSession(context *gin.Context) (bool, *UACClaims) {
	session := sessions.DefaultMany(context, "user_session")
	jwtToken := session.Get(JWT_TOKEN_KEY)

	if jwtToken == nil {
		return false, nil
	}

	claim, err := auth.JWTCrypto.DecryptJWT(jwtToken)
	if err != nil || claim == nil {
		return false, nil
	}
	return true, claim
}

func (auth *Auth) Login(context *gin.Context, session sessions.Session) {
	var uacLength = 12
	uac := context.PostForm("uac")
	uac = strings.ReplaceAll(uac, " ", "")

	if uac == "" {
		auth.Logger.Info("Failed Login", append(utils.GetRequestSource(context),
			zap.String("Reason", "Blank UAC"))...)
		auth.NotAuthWithError(context, auth.uacError(context))
		return
	}

	if auth.isUac16() {
		uacLength = 16
	}

	if len(uac) != uacLength {
		auth.Logger.Info("Failed Login", append(utils.GetRequestSource(context),
			zap.String("Reason", "Invalid UAC length"), zap.Int("UACLength", uacLength))...)
		auth.NotAuthWithError(context, auth.uacError(context))
		return
	}

	uacInfo, err := auth.BusApi.GetUacInfo(uac)
	if err != nil || uacInfo.InvalidCase() {
		auth.Logger.Info("Failed Login", append(utils.GetRequestSource(context),
			zap.String("Reason", "Access code not recognised"),
			zap.String("InstrumentName", uacInfo.InstrumentName),
			zap.String("CaseID", uacInfo.CaseID),
			zap.Error(err),
		)...)
		auth.NotAuthWithError(context, auth.LanguageManager.LanguageError(NOT_RECOGNISED_ERR, context))
		return
	}

	instrumentSettings, err := auth.BlaiseRestApi.GetInstrumentSettings(uacInfo.InstrumentName)
	if err != nil {
		if err == blaiserestapi.InstrumentNotFoundError {
			auth.Logger.Warn("Failed Login", append(utils.GetRequestSource(context),
				zap.String("Reason", "Instrument not installed"),
				zap.String("Notes", "This can happen if a UAC for a non-Blaise 5 survey has been entered"),
				zap.String("InstrumentName", uacInfo.InstrumentName),
				zap.String("CaseID", uacInfo.CaseID),
				zap.Error(err),
			)...)
			auth.InstrumentNotInstalledError(context)
			return
		}
		auth.Logger.Error("Failed Login", append(utils.GetRequestSource(context),
			zap.String("Reason", "Could not get instrument settings"),
			zap.String("InstrumentName", uacInfo.InstrumentName),
			zap.String("CaseID", uacInfo.CaseID),
			zap.Error(err),
		)...)
		auth.NotAuthWithError(context, auth.LanguageManager.LanguageError(INTERNAL_SERVER_ERR, context))
		return
	}

	sessionTimeout := instrumentSettings.StrictInterviewing().SessionTimeout
	if sessionTimeout == 0 {
		sessionTimeout = DefaultAuthTimeout
	}
	signedToken, err := auth.JWTCrypto.EncryptJWT(uac, &uacInfo, sessionTimeout)
	if err != nil {
		auth.Logger.Error("Failed to Encrypt JWT", zap.Error(err))
		auth.NotAuthWithError(context, auth.LanguageManager.LanguageError(INTERNAL_SERVER_ERR, context))
		return
	}

	session.Set(JWT_TOKEN_KEY, signedToken)
	session.Set(SESSION_TIMEOUT_KEY, sessionTimeout)
	if err := session.Save(); err != nil {
		auth.Logger.Error("Failed to save JWT to session", zap.Error(err))
		auth.NotAuthWithError(context, auth.LanguageManager.LanguageError(INTERNAL_SERVER_ERR, context))
		return
	}

	validationSession := sessions.DefaultMany(context, "session_validation")
	validationSession.Set(SESSION_VALID_KEY, true)
	if err := validationSession.Save(); err != nil {
		auth.Logger.Error("Failed to save validationSession", zap.Error(err))
		auth.NotAuthWithError(context, auth.LanguageManager.LanguageError(INTERNAL_SERVER_ERR, context))
		return
	}

    auth.Logger.Info(fmt.Sprintf("Successful Login with InstrumentName: %s",
    uacInfo.InstrumentName),
	    append(utils.GetRequestSource(context),
			zap.String("InstrumentName", uacInfo.InstrumentName),
			zap.String("CaseID", uacInfo.CaseID),
			)...)

	context.Redirect(http.StatusFound, fmt.Sprintf("/%s/", uacInfo.InstrumentName))
	context.Abort()
}

func (auth *Auth) Logout(context *gin.Context, session sessions.Session) {
	session.Set(JWT_TOKEN_KEY, "")
	session.Clear()
	session.Options(sessions.Options{MaxAge: -1})
	err := session.Save()
	if err != nil || auth.clearSessionValidation(context) != nil {
		auth.notAuth(context)
		return
	}
	context.HTML(http.StatusOK, "logout.tmpl", gin.H{"welsh": auth.LanguageManager.IsWelsh(context)})
}

func (auth *Auth) notAuth(context *gin.Context) {
	context.HTML(http.StatusUnauthorized, "login.tmpl", gin.H{
		"uac16":      auth.isUac16(),
		"csrf_token": auth.CSRFManager.GetToken(context),
		"welsh":      auth.LanguageManager.IsWelsh(context),
	})
	context.Abort()
}

func (auth *Auth) NotAuthWithError(context *gin.Context, errorMessage string) {
	context.HTML(http.StatusUnauthorized, "login.tmpl", gin.H{
		"error":      errorMessage,
		"uac16":      auth.isUac16(),
		"csrf_token": auth.CSRFManager.GetToken(context),
		"welsh":      auth.LanguageManager.IsWelsh(context),
	})
	context.Abort()
}

func (auth *Auth) InstrumentNotInstalledError(context *gin.Context) {
	context.HTML(http.StatusOK, "not_live.tmpl", gin.H{"welsh": auth.LanguageManager.IsWelsh(context)})
	context.Abort()
}

func (auth *Auth) RefreshToken(context *gin.Context, session sessions.Session, claim *UACClaims) {
	jwtToken := session.Get(JWT_TOKEN_KEY)
	if jwtToken == nil || jwtToken.(string) == "" ||
		!auth.SessionValid(context) {
		auth.Logger.Info("Not refreshing JWT as it looks like the user has logged out",
			append(utils.GetRequestSource(context),
				zap.String("InstrumentName", claim.UacInfo.InstrumentName),
				zap.String("CaseID", claim.UacInfo.InstrumentName),
			)...)
		return
	}

	signedToken, err := auth.JWTCrypto.EncryptJWT(claim.UAC, &claim.UacInfo, claim.AuthTimeout)
	if err != nil {
		auth.Logger.Error("Failed to Encrypt JWT", zap.Error(err))
		return
	}

	session.Set(JWT_TOKEN_KEY, signedToken)
	if err := session.Save(); err != nil {
		auth.Logger.Error("Failed to save JWT to session", zap.Error(err))
		return
	}
}

func (auth *Auth) SessionValid(context *gin.Context) bool {
	validationSession := sessions.DefaultMany(context, "session_validation")
	sessionValid := validationSession.Get(SESSION_VALID_KEY)
	if sessionValid == nil {
		return false
	}
	return sessionValid.(bool)
}

func (auth *Auth) clearSessionValidation(context *gin.Context) error {
	validationSession := sessions.DefaultMany(context, "session_validation")
	validationSession.Set(SESSION_VALID_KEY, false)
	validationSession.Clear()
	validationSession.Options(sessions.Options{MaxAge: -1})
	return validationSession.Save()
}

func (auth *Auth) isUac16() bool {
	return auth.UacKind == "uac16"
}

func (auth *Auth) uacError(context *gin.Context) string {
	if auth.LanguageManager.IsWelsh(context) {
		if auth.isUac16() {
			return fmt.Sprintf(INVALID_LENGTH_ERR["welsh"], "16 o nodau")
		}
		return fmt.Sprintf(INVALID_LENGTH_ERR["welsh"], "12 o nodau")
	}
	if auth.isUac16() {
		return fmt.Sprintf(INVALID_LENGTH_ERR["english"], "16-character")
	}
	return fmt.Sprintf(INVALID_LENGTH_ERR["english"], "12-digit")
}

func Forbidden(context *gin.Context, welsh bool) {
	context.HTML(http.StatusForbidden, "access_denied.tmpl", gin.H{"welsh": welsh})
	context.Abort()
}
