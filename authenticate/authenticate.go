package authenticate

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/blaiserestapi"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/ONSdigital/blaise-cawi-portal/csrf"
	"github.com/ONSdigital/blaise-cawi-portal/languagemanager"
	"github.com/ONSdigital/blaise-cawi-portal/sessionkeys"
	"github.com/ONSdigital/blaise-cawi-portal/utils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
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
	CSRF_ERR = map[string]string{
		"english": "Request timed out, please try again",
		"welsh":   "Cais wedi dod i ben, triwch eto",
	}
)

//go:generate mockery
type AuthInterface interface {
	AuthenticatedWithUAC(*gin.Context)
	IsUAC16() bool
	Login(*gin.Context, sessions.Session)
	Logout(*gin.Context, sessions.Session)
	HasSession(*gin.Context) (bool, *UACClaims)
	NotAuthWithError(*gin.Context, string)
	RefreshToken(*gin.Context, sessions.Session, *UACClaims)
}

type Auth struct {
	BUSAPI          busapi.BUSAPIInterface
	JWTCrypto       JWTCryptoInterface
	BlaiseRestAPI   blaiserestapi.BlaiseRestAPIInterface
	Logger          *zap.Logger
	UACKind         string
	CSRFManager     csrf.CSRFManager
	LanguageManager languagemanager.LanguageManagerInterface
}

func (auth *Auth) logger() *zap.Logger {
	if auth.Logger != nil {
		return auth.Logger
	}
	return zap.L()
}

func (auth *Auth) AuthenticatedWithUAC(context *gin.Context) {
	session := sessions.DefaultMany(context, sessionkeys.UserSessionName)
	jwtToken := session.Get(JWT_TOKEN_KEY)

	if jwtToken == nil || !auth.SessionValid(context) {
		auth.notAuthed(context)
		return
	}

	_, err := auth.JWTCrypto.DecryptJWT(jwtToken)
	if err != nil {
		auth.logger().Warn("Failed to decrypt JWT from session", append(utils.GetRequestSource(context), zap.Error(err))...)
		auth.notAuthed(context)
		return
	}
	context.Next()
}

func (auth *Auth) HasSession(context *gin.Context) (bool, *UACClaims) {
	session := sessions.DefaultMany(context, sessionkeys.UserSessionName)
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
		auth.logger().Info("Failed auth", append(utils.GetRequestSource(context),
			zap.String("Reason", "Blank UAC"))...)
		auth.NotAuthWithError(context, auth.uacError(context))
		return
	}

	if auth.IsUAC16() {
		uacLength = 16
	}

	if len(uac) != uacLength {
		auth.logger().Info("Failed auth", append(utils.GetRequestSource(context),
			zap.String("Reason", "Invalid UAC length"), zap.Int("UACLength", uacLength))...)
		auth.NotAuthWithError(context, auth.uacError(context))
		return
	}

	uacInfo, err := auth.BUSAPI.GetUACInfo(context.Request.Context(), uac)

	if err != nil {
		auth.logger().Error("Failed auth", append(utils.GetRequestSource(context),
			zap.String("Reason", "Error retrieving UAC information"),
			zap.Error(err),
		)...)

		auth.NotAuthWithError(context, auth.LanguageManager.LanguageError(INTERNAL_SERVER_ERR, context))
		return
	}

	if uacInfo.InvalidCase() {
		auth.logger().Info("Failed auth", append(utils.GetRequestSource(context),
			zap.String("Reason", "Access code not recognised"),
			zap.String("InstrumentName", uacInfo.InstrumentName),
			zap.String("CaseIDFingerprint", CaseIDFingerprint(uacInfo.CaseID)),
		)...)

		auth.NotAuthWithError(context, auth.LanguageManager.LanguageError(NOT_RECOGNISED_ERR, context))
		return
	}

	instrumentSettings, err := auth.BlaiseRestAPI.GetInstrumentSettings(context.Request.Context(), uacInfo.InstrumentName)
	if err != nil {
		if errors.Is(err, blaiserestapi.InstrumentNotFoundError) {
			auth.logger().Warn("Failed auth", append(utils.GetRequestSource(context),
				zap.String("Reason", "Instrument not installed"),
				zap.String("Notes", "This can happen if a UAC for a non-Blaise 5 survey has been entered"),
				zap.String("InstrumentName", uacInfo.InstrumentName),
				zap.String("CaseIDFingerprint", CaseIDFingerprint(uacInfo.CaseID)),
				zap.Error(err),
			)...)
			auth.InstrumentNotInstalledError(context)
			return
		}
		auth.logger().Error("Failed auth", append(utils.GetRequestSource(context),
			zap.String("Reason", "Could not get instrument settings"),
			zap.String("InstrumentName", uacInfo.InstrumentName),
			zap.String("CaseIDFingerprint", CaseIDFingerprint(uacInfo.CaseID)),
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
		auth.logger().Error("Failed to encrypt JWT", zap.Error(err))
		auth.NotAuthWithError(context, auth.LanguageManager.LanguageError(INTERNAL_SERVER_ERR, context))
		return
	}

	session.Set(JWT_TOKEN_KEY, signedToken)
	session.Set(SESSION_TIMEOUT_KEY, sessionTimeout)
	if err := session.Save(); err != nil {
		auth.logger().Error("Failed to save JWT to session", zap.Error(err))
		auth.NotAuthWithError(context, auth.LanguageManager.LanguageError(INTERNAL_SERVER_ERR, context))
		return
	}

	validationSession := sessions.DefaultMany(context, sessionkeys.SessionValidationName)
	validationSession.Set(SESSION_VALID_KEY, true)
	if err := validationSession.Save(); err != nil {
		auth.logger().Error("Failed to save validation session", zap.Error(err))
		auth.NotAuthWithError(context, auth.LanguageManager.LanguageError(INTERNAL_SERVER_ERR, context))
		return
	}

	instrumentName := utils.SanitiseLogInput(uacInfo.InstrumentName)
	caseID := utils.SanitiseLogInput(uacInfo.CaseID)

	auth.logger().Info(fmt.Sprintf("Successful auth with questionnaire: %s, case ID: %s", instrumentName, caseID),
		append(utils.GetRequestSource(context),
			zap.String("InstrumentName", instrumentName),
			zap.String("CaseID", caseID),
		)...)

	context.Redirect(http.StatusFound, fmt.Sprintf("/%s/", uacInfo.InstrumentName))
	context.Abort()
}

func (auth *Auth) Logout(context *gin.Context, session sessions.Session) {
	session.Set(JWT_TOKEN_KEY, "")
	session.Clear()
	session.Options(sessions.Options{MaxAge: -1})
	saveErr := session.Save()
	clearErr := auth.clearSessionValidation(context)
	if saveErr != nil || clearErr != nil {
		auth.notAuthed(context)
		return
	}
	context.HTML(http.StatusOK, "logout.tmpl", gin.H{"welsh": auth.LanguageManager.IsWelsh(context)})
}

func (auth *Auth) notAuthed(context *gin.Context) {
	context.HTML(http.StatusUnauthorized, "login.tmpl", gin.H{
		"uac16":      auth.IsUAC16(),
		"csrf_token": auth.CSRFManager.GetToken(context),
		"welsh":      auth.LanguageManager.IsWelsh(context),
	})
	context.Abort()
}

func (auth *Auth) NotAuthWithError(context *gin.Context, errorMessage string) {
	context.HTML(http.StatusUnauthorized, "login.tmpl", gin.H{
		"error":      errorMessage,
		"uac16":      auth.IsUAC16(),
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
	jwtTokenValue := session.Get(JWT_TOKEN_KEY)
	jwtToken, tokenTypeOk := jwtTokenValue.(string)
	if jwtTokenValue == nil || !tokenTypeOk || jwtToken == "" ||
		!auth.SessionValid(context) {
		auth.logger().Info("Not refreshing JWT as it looks like the user has logged out",
			append(utils.GetRequestSource(context),
				zap.String("InstrumentName", claim.UACInfo.InstrumentName),
				zap.String("CaseIDFingerprint", CaseIDFingerprint(claim.UACInfo.CaseID)),
			)...)
		return
	}

	signedToken, err := auth.JWTCrypto.EncryptJWT(claim.UAC, &claim.UACInfo, claim.AuthTimeout)
	if err != nil {
		auth.logger().Error("Failed to encrypt JWT", zap.Error(err))
		return
	}

	session.Set(JWT_TOKEN_KEY, signedToken)
	if err := session.Save(); err != nil {
		auth.logger().Error("Failed to save JWT to session", zap.Error(err))
		return
	}
}

func (auth *Auth) SessionValid(context *gin.Context) bool {
	validationSession := sessions.DefaultMany(context, sessionkeys.SessionValidationName)
	sessionValid := validationSession.Get(SESSION_VALID_KEY)
	if sessionValid == nil {
		return false
	}
	sessionValidBool, ok := sessionValid.(bool)
	if !ok {
		return false
	}

	return sessionValidBool
}

func (auth *Auth) clearSessionValidation(context *gin.Context) error {
	validationSession := sessions.DefaultMany(context, sessionkeys.SessionValidationName)
	validationSession.Set(SESSION_VALID_KEY, false)
	validationSession.Clear()
	validationSession.Options(sessions.Options{MaxAge: -1})
	return validationSession.Save()
}

func (auth *Auth) IsUAC16() bool {
	return auth.UACKind == "uac16"
}

func (auth *Auth) uacError(context *gin.Context) string {
	if auth.LanguageManager.IsWelsh(context) {
		if auth.IsUAC16() {
			return fmt.Sprintf(INVALID_LENGTH_ERR["welsh"], "16 o nodau")
		}
		return fmt.Sprintf(INVALID_LENGTH_ERR["welsh"], "12 o nodau")
	}
	if auth.IsUAC16() {
		return fmt.Sprintf(INVALID_LENGTH_ERR["english"], "16-character")
	}
	return fmt.Sprintf(INVALID_LENGTH_ERR["english"], "12-digit")
}

func Forbidden(context *gin.Context, welsh bool) {
	context.HTML(http.StatusForbidden, "access_denied.tmpl", gin.H{"welsh": welsh})
	context.Abort()
}
