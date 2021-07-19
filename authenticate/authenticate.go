package authenticate

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

const (
	JWT_TOKEN_KEY       = "jwt_token"
	NO_ACCESS_CODE_ERR  = "Enter an access code"
	INVALID_LENGTH_ERR  = "Enter a 12-character access code"
	NOT_RECOGNISED_ERR  = "Access code not recognised. Enter the code again"
	INTERNAL_SERVER_ERR = "We were unable to process your request, please try again"
	ISSUER              = "social-surveys-web-portal"
)

// var expirationTime = "2h"
var expirationTime = "30s"

//Generate mocks by running "go generate ./..."
//go:generate mockery --name AuthInterface
type AuthInterface interface {
	Authenticated(*gin.Context)
	Login(*gin.Context, sessions.Session)
	Logout(*gin.Context, sessions.Session)
	DecryptJWT(interface{}) (*UACClaims, error)
}

type Auth struct {
	JWTSecret string
	BusApi    busapi.BusApiInterface
}

func (auth *Auth) Authenticated(context *gin.Context) {
	session := sessions.Default(context)
	jwtToken := session.Get(JWT_TOKEN_KEY)

	if jwtToken == nil {
		notAuth(context)
		return
	}

	_, err := auth.DecryptJWT(jwtToken)
	if err != nil {
		log.Println(err)
		notAuth(context)
		return
	}
	context.Next()
}

func (auth *Auth) Login(context *gin.Context, session sessions.Session) {
	uac := context.PostForm("uac")

	if uac == "" {
		notAuthWithError(context, NO_ACCESS_CODE_ERR)
		return
	}
	if len(uac) <= 11 || len(uac) >= 13 {
		notAuthWithError(context, INVALID_LENGTH_ERR)
		return
	}

	uacInfo, err := auth.BusApi.GetUacInfo(uac)
	if err != nil || uacInfo.InstrumentName == "" || uacInfo.CaseID == "" {
		notAuthWithError(context, NOT_RECOGNISED_ERR)
		return
	}

	signedToken, err := auth.encryptJWT(uac, &uacInfo)
	if err != nil {
		log.Println(err)
		notAuthWithError(context, INTERNAL_SERVER_ERR)
		return
	}

	session.Set(JWT_TOKEN_KEY, signedToken)
	if err := session.Save(); err != nil {
		log.Println(err)
		notAuthWithError(context, INTERNAL_SERVER_ERR)
		return
	}
	context.Redirect(http.StatusMovedPermanently, fmt.Sprintf("/%s/", uacInfo.InstrumentName))
	context.Abort()
}

func (auth *Auth) encryptJWT(uac string, uacInfo *busapi.UacInfo) (string, error) {
	claims := UACClaims{
		UAC: uac,
		UacInfo: busapi.UacInfo{
			InstrumentName: uacInfo.InstrumentName,
			CaseID:         uacInfo.CaseID,
		},
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Unix() + expirationSeconds(),
			Issuer:    ISSUER,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(auth.JWTSecret))
}

func (auth *Auth) DecryptJWT(jwtToken interface{}) (*UACClaims, error) {
	if jwtToken == nil {
		return nil, fmt.Errorf("No JWT Token in session")
	}
	token, err := jwt.ParseWithClaims(jwtToken.(string), &UACClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(auth.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	if err := token.Claims.Valid(); err != nil {
		return nil, err
	}

	return token.Claims.(*UACClaims), nil
}

func (auth *Auth) Logout(context *gin.Context, session sessions.Session) {
	session.Clear()
	err := session.Save()
	if err != nil {
		notAuth(context)
		return
	}
	context.HTML(http.StatusOK, "login.tmpl", gin.H{})
	context.Abort()
}

func notAuth(context *gin.Context) {
	context.HTML(http.StatusUnauthorized, "login.tmpl", gin.H{})
	context.Abort()
}

func notAuthWithError(context *gin.Context, errorMessage string) {
	context.HTML(http.StatusUnauthorized, "login.tmpl", gin.H{"error": errorMessage})
	context.Abort()
}

func expirationSeconds() int64 {
	duration, _ := time.ParseDuration(expirationTime)
	return int64(duration.Seconds())
}
