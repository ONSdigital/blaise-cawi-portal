package authenticate

import (
	"fmt"
	"time"

	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/golang-jwt/jwt/v5"
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

var DefaultAuthTimeout = 15

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
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(authTimeout) * time.Minute)),
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
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtCrypto.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*UACClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
