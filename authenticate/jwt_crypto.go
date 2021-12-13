package authenticate

import (
	"fmt"
	"time"

	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/golang-jwt/jwt"
)

//Generate mocks by running "go generate ./..."
//go:generate mockery --name JWTCryptoInterface
type JWTCryptoInterface interface {
	EncryptJWT(string, *busapi.UacInfo, int) (string, error)
	DecryptJWT(interface{}) (*UACClaims, error)
}

type JWTCrypto struct {
	JWTSecret string
}

var DefaultAuthTimeout = 1

func (jwtCrypto *JWTCrypto) EncryptJWT(uac string, uacInfo *busapi.UacInfo, authTimeout int) (string, error) {
	if authTimeout == 0 {
		authTimeout = DefaultAuthTimeout
	}

	claims := UACClaims{
		UAC:         uac,
		AuthTimeout: authTimeout,
		UacInfo: busapi.UacInfo{
			InstrumentName: uacInfo.InstrumentName,
			CaseID:         uacInfo.CaseID,
		},
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Unix() + expirationSeconds(authTimeout),
			Issuer:    ISSUER,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtCrypto.JWTSecret))
}

func (jwtCrypto *JWTCrypto) DecryptJWT(jwtToken interface{}) (*UACClaims, error) {
	if jwtToken == nil {
		return nil, fmt.Errorf("no JWT Token in session")
	}
	token, err := jwt.ParseWithClaims(jwtToken.(string), &UACClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtCrypto.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	if err := token.Claims.Valid(); err != nil {
		return nil, err
	}

	return token.Claims.(*UACClaims), nil
}

func expirationSeconds(sessionTimeout int) int64 {
	sessionMinutes := time.Duration(sessionTimeout) * time.Minute
	return int64(sessionMinutes.Seconds())
}
