package authenticate

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/blaiserestapi"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/ONSdigital/blaise-cawi-portal/utils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	csrf "github.com/utrack/gin-csrf"
	"go.uber.org/zap"
)

const (
	SESSION_TIMEOUT_KEY = "session_timeout"
	JWT_TOKEN_KEY       = "jwt_token"
	NO_ACCESS_CODE_ERR  = "Enter an access code"
	INVALID_LENGTH_ERR  = "Enter a %s access code"
	NOT_RECOGNISED_ERR  = "Access code not recognised. Enter the code again"
	INTERNAL_SERVER_ERR = "We were unable to process your request, please try again"
	ISSUER              = "social-surveys-web-portal"
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
	BusApi        busapi.BusApiInterface
	JWTCrypto     JWTCryptoInterface
	BlaiseRestApi blaiserestapi.BlaiseRestApiInterface
	Logger        *zap.Logger
	CSRFSecret    string
	UacKind       string
}

func (auth *Auth) AuthenticatedWithUac(context *gin.Context) {
	session := sessions.Default(context)
	jwtToken := session.Get(JWT_TOKEN_KEY)

	if jwtToken == nil {
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
	session := sessions.Default(context)
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
		auth.Logger.Info("Failed auth", append(utils.GetRequestSource(context),
			zap.String("Reason", "Blank UAC"))...)
		auth.NotAuthWithError(context, NO_ACCESS_CODE_ERR)
		return
	}

	if auth.isUac16() {
		uacLength = 16
	}

	if len(uac) != uacLength {
		auth.Logger.Info("Failed auth", append(utils.GetRequestSource(context),
			zap.String("Reason", "Invalid UAC length"), zap.Int("UACLength", uacLength))...)
		auth.NotAuthWithError(context, fmt.Sprintf(INVALID_LENGTH_ERR, auth.uacError()))
		return
	}

	uacInfo, err := auth.BusApi.GetUacInfo(uac)
	if err != nil || uacInfo.InvalidCase() {
		auth.Logger.Info("Failed auth", append(utils.GetRequestSource(context),
			zap.String("Reason", "Access code not recognised"),
			zap.String("InstrumentName", uacInfo.InstrumentName),
			zap.String("CaseID", uacInfo.CaseID),
			zap.Error(err),
		)...)
		auth.NotAuthWithError(context, NOT_RECOGNISED_ERR)
		return
	}

	instrumentSettings, err := auth.BlaiseRestApi.GetInstrumentSettings(uacInfo.InstrumentName)
	if err != nil {
		auth.Logger.Error("Failed auth", append(utils.GetRequestSource(context),
			zap.String("Reason", "Could not get instrument settings"),
			zap.String("InstrumentName", uacInfo.InstrumentName),
			zap.String("CaseID", uacInfo.CaseID),
			zap.Error(err),
		)...)
		auth.NotAuthWithError(context, INTERNAL_SERVER_ERR)
		return
	}

	sessionTimeout := instrumentSettings.StrictInterviewing().SessionTimeout
	if sessionTimeout == 0 {
		sessionTimeout = DefaultAuthTimeout
	}
	signedToken, err := auth.JWTCrypto.EncryptJWT(uac, &uacInfo, sessionTimeout)
	if err != nil {
		auth.Logger.Error("Failed to Encrypt JWT", zap.Error(err))
		auth.NotAuthWithError(context, INTERNAL_SERVER_ERR)
		return
	}

	session.Set(JWT_TOKEN_KEY, signedToken)
	session.Set(SESSION_TIMEOUT_KEY, sessionTimeout)
	if err := session.Save(); err != nil {
		auth.Logger.Error("Failed to save JWT to session", zap.Error(err))
		auth.NotAuthWithError(context, INTERNAL_SERVER_ERR)
		return
	}

	context.Redirect(http.StatusFound, fmt.Sprintf("/%s/", uacInfo.InstrumentName))
	context.Abort()
}

func (auth *Auth) Logout(context *gin.Context, session sessions.Session) {
	session.Clear()
	err := session.Save()
	if err != nil {
		auth.notAuth(context)
		return
	}
	context.HTML(http.StatusOK, "logout.tmpl", gin.H{})
	context.Abort()
}

func (auth *Auth) notAuth(context *gin.Context) {
	context.Set("csrfSecret", auth.CSRFSecret)
	context.HTML(http.StatusUnauthorized, "login.tmpl", gin.H{
		"uac16":      auth.isUac16(),
		"csrf_token": csrf.GetToken(context)})
	context.Abort()
}

func (auth *Auth) NotAuthWithError(context *gin.Context, errorMessage string) {
	context.Set("csrfSecret", auth.CSRFSecret)
	context.HTML(http.StatusUnauthorized, "login.tmpl", gin.H{
		"error":      errorMessage,
		"uac16":      auth.isUac16(),
		"csrf_token": csrf.GetToken(context)})
	context.Abort()
}

func (auth *Auth) isUac16() bool {
	return auth.UacKind == "uac16"
}

func (auth *Auth) uacError() string {
	if auth.isUac16() {
		return "16-character"
	}
	return "12-digit"
}

func Forbidden(context *gin.Context) {
	context.HTML(http.StatusForbidden, "access_denied.tmpl", gin.H{})
	context.Abort()
}

func (auth *Auth) RefreshToken(context *gin.Context, session sessions.Session, claim *UACClaims) {
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
	return
}
