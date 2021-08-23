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
	EncryptJWT(string, *busapi.UacInfo) (string, error)
	EncryptValidatedPostcodeJWT(*UACClaims) (string, error)
	DecryptJWT(interface{}) (*UACClaims, error)
}

type JWTCrypto struct {
	JWTSecret string
}

func (jwtCrypto *JWTCrypto) EncryptJWT(uac string, uacInfo *busapi.UacInfo) (string, error) {
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
	return token.SignedString([]byte(jwtCrypto.JWTSecret))
}

func (jwtCrypto *JWTCrypto) EncryptValidatedPostcodeJWT(claim *UACClaims) (string, error) {
	claim.PostcodeValidated = true

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)
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
