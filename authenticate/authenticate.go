package authenticate

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ONSdigital/blaise-cawi-portal/blaiserestapi"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	JWT_TOKEN_KEY             = "jwt_token"
	NO_ACCESS_CODE_ERR        = "Enter an access code"
	INVALID_LENGTH_ERR        = "Enter a 12-character access code"
	NOT_RECOGNISED_ERR        = "Access code not recognised. Enter the code again"
	INTERNAL_SERVER_ERR       = "We were unable to process your request, please try again"
	POSTCODE_VALIDATION_ERR   = "Postcode not recognised, please try again"
	POSTCODE_ATTEMPT_ERR      = "This uac is currently locked due to repeated failed postcode attempts"
	ISSUER                    = "social-surveys-web-portal"
	MAX_POSTCODE_ATTEMPTS     = 5
	POSTCODE_ATTEMPT_TIMEOUT  = time.Duration(30 * time.Minute)
	POSTCODE_TIMESTAMP_FORMAT = "2006-01-02 15:04:05.999999999 -0700 MST"
)

var expirationTime = "2h"

// var expirationTime = "30s"

//Generate mocks by running "go generate ./..."
//go:generate mockery --name AuthInterface
type AuthInterface interface {
	AuthenticatedWithUac(*gin.Context)
	AuthenticatedWithUacAndPostcode(*gin.Context)
	Login(*gin.Context, sessions.Session)
	LoginPostcode(*gin.Context, sessions.Session)
	Logout(*gin.Context, sessions.Session)
	HasSession(*gin.Context) (bool, *UACClaims)
}

type Auth struct {
	BusApi        busapi.BusApiInterface
	JWTCrypto     JWTCryptoInterface
	BlaiseRestApi blaiserestapi.BlaiseRestApiInterface
}

func (auth *Auth) AuthenticatedWithUac(context *gin.Context) {
	session := sessions.Default(context)
	jwtToken := session.Get(JWT_TOKEN_KEY)

	if jwtToken == nil {
		notAuth(context)
		return
	}

	_, err := auth.JWTCrypto.DecryptJWT(jwtToken)
	if err != nil {
		log.Println(err)
		notAuth(context)
		return
	}
	context.Next()
}

func (auth *Auth) AuthenticatedWithUacAndPostcode(context *gin.Context) {
	session := sessions.Default(context)
	jwtToken := session.Get(JWT_TOKEN_KEY)

	if jwtToken == nil {
		notAuth(context)
		return
	}

	claim, err := auth.JWTCrypto.DecryptJWT(jwtToken)
	if err != nil {
		log.Println(err)
		notAuth(context)
		return
	}
	if claim == nil || !claim.PostcodeValidated {
		notAuth(context)
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
	if err != nil || claim == nil || !claim.PostcodeValidated {
		return false, nil
	}
	return true, claim
}

func (auth *Auth) Login(context *gin.Context, session sessions.Session) {
	uac := context.PostForm("uac")
	uac = strings.ReplaceAll(uac, " ", "")

	if uac == "" {
		NotAuthWithError(context, NO_ACCESS_CODE_ERR)
		return
	}
	if len(uac) <= 11 || len(uac) >= 13 {
		NotAuthWithError(context, INVALID_LENGTH_ERR)
		return
	}

	uacInfo, err := auth.BusApi.GetUacInfo(uac)
	if err != nil || uacInfo.InstrumentName == "" || uacInfo.CaseID == "" {
		NotAuthWithError(context, NOT_RECOGNISED_ERR)
		return
	}

	if uacInfo.PostcodeAttempts >= MAX_POSTCODE_ATTEMPTS {
		postcodeAttemptTimestamp, err := time.Parse(POSTCODE_TIMESTAMP_FORMAT, uacInfo.PostcodeAttemptTimestamp)
		if err != nil {
			log.Println(err)
			NotAuthWithError(context, INTERNAL_SERVER_ERR)
			return
		}
		if time.Now().UTC().Before(postcodeAttemptTimestamp.Add(POSTCODE_ATTEMPT_TIMEOUT)) {
			NotAuthWithError(context, POSTCODE_ATTEMPT_ERR)
			return
		}
	}

	signedToken, err := auth.JWTCrypto.EncryptJWT(uac, &uacInfo)
	if err != nil {
		log.Println(err)
		NotAuthWithError(context, INTERNAL_SERVER_ERR)
		return
	}

	session.Set(JWT_TOKEN_KEY, signedToken)
	if err := session.Save(); err != nil {
		log.Println(err)
		NotAuthWithError(context, INTERNAL_SERVER_ERR)
		return
	}
	context.Redirect(http.StatusFound, "/auth/login/postcode")
	context.Abort()
}

func (auth *Auth) LoginPostcode(context *gin.Context, session sessions.Session) {
	enteredPostcode := context.PostForm("postcode")

	jwtToken := session.Get(JWT_TOKEN_KEY)

	if jwtToken == nil {
		notAuth(context)
		return
	}

	claim, err := auth.JWTCrypto.DecryptJWT(jwtToken)
	if err != nil {
		log.Println(err)
		notAuth(context)
		return
	}

	uacInfo, err := auth.BusApi.GetUacInfo(claim.UAC)
	if err != nil {
		log.Println(err)
		NotAuthPostcodeWithError(context, INTERNAL_SERVER_ERR)
		return
	}

	if uacInfo.PostcodeAttempts > 0 {
		postcodeAttemptTimestamp, err := time.Parse(POSTCODE_TIMESTAMP_FORMAT, uacInfo.PostcodeAttemptTimestamp)
		if err != nil {
			log.Println(err)
			NotAuthPostcodeWithError(context, INTERNAL_SERVER_ERR)
			return
		}
		if time.Now().UTC().After(postcodeAttemptTimestamp.Add(POSTCODE_ATTEMPT_TIMEOUT)) {
			uacInfo, err = auth.BusApi.ResetPostcodeAttempts(claim.UAC)
			if err != nil {
				log.Println(err)
				NotAuthPostcodeWithError(context, INTERNAL_SERVER_ERR)
				return
			}
		}
	}

	if uacInfo.PostcodeAttempts >= MAX_POSTCODE_ATTEMPTS {
		NotAuthWithError(context, POSTCODE_ATTEMPT_ERR)
		return
	}

	casePostcode, err := auth.BlaiseRestApi.GetPostCode(claim.UacInfo.InstrumentName, claim.UacInfo.CaseID)
	if err != nil {
		log.Println(err)
		NotAuthPostcodeWithError(context, INTERNAL_SERVER_ERR)
		return
	}
	if !auth.ValidatePostcode(enteredPostcode, casePostcode) {
		uacInfo, err := auth.BusApi.IncrementPostcodeAttempts(claim.UAC)
		if err != nil {
			log.Println(err)
			NotAuthPostcodeWithError(context, INTERNAL_SERVER_ERR)
			return
		}
		if uacInfo.PostcodeAttempts >= MAX_POSTCODE_ATTEMPTS {
			NotAuthWithError(context, POSTCODE_ATTEMPT_ERR)
			return
		}
		NotAuthPostcodeWithError(context, POSTCODE_VALIDATION_ERR)
		return
	}

	signedToken, err := auth.JWTCrypto.EncryptValidatedPostcodeJWT(claim)
	if err != nil {
		log.Println(err)
		NotAuthPostcodeWithError(context, INTERNAL_SERVER_ERR)
		return
	}

	session.Set(JWT_TOKEN_KEY, signedToken)
	if err := session.Save(); err != nil {
		log.Println(err)
		NotAuthPostcodeWithError(context, INTERNAL_SERVER_ERR)
		return
	}
	if uacInfo.PostcodeAttempts > 0 {
		_, err = auth.BusApi.ResetPostcodeAttempts(claim.UAC)
		if err != nil {
			log.Println(err)
			NotAuthPostcodeWithError(context, INTERNAL_SERVER_ERR)
			return
		}
	}
	context.Redirect(http.StatusFound, fmt.Sprintf("/%s/", claim.UacInfo.InstrumentName))
	context.Abort()
}

func (auth *Auth) Logout(context *gin.Context, session sessions.Session) {
	session.Clear()
	err := session.Save()
	if err != nil {
		notAuth(context)
		return
	}
	context.HTML(http.StatusOK, "logout.tmpl", gin.H{})
	context.Abort()
}

// Checks for equality of an entered postcode, vs that in a case, ignoring case and whitespace
func (auth *Auth) ValidatePostcode(enteredPostcode, casePostcode string) bool {
	enteredPostcode = strings.ReplaceAll(enteredPostcode, " ", "")
	casePostcode = strings.ReplaceAll(casePostcode, " ", "")
	return strings.ToLower(enteredPostcode) == strings.ToLower(casePostcode)
}

func notAuth(context *gin.Context) {
	context.HTML(http.StatusUnauthorized, "login.tmpl", gin.H{})
	context.Abort()
}

func NotAuthWithError(context *gin.Context, errorMessage string) {
	context.HTML(http.StatusUnauthorized, "login.tmpl", gin.H{"error": errorMessage})
	context.Abort()
}

func NotAuthPostcodeWithError(context *gin.Context, errorMessage string) {
	context.HTML(http.StatusUnauthorized, "postcode.tmpl", gin.H{"error": errorMessage})
	context.Abort()
}

func Forbidden(context *gin.Context) {
	context.HTML(http.StatusForbidden, "access_denied.tmpl", gin.H{})
	context.Abort()
}

func expirationSeconds() int64 {
	duration, _ := time.ParseDuration(expirationTime)
	return int64(duration.Seconds())
}
