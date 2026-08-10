package authenticate

import (
	"fmt"
	"time"

	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/golang-jwt/jwt/v5"
)

//go:generate mockery --name JWTCryptoInterface
type JWTCryptoInterface interface {
	EncryptJWT(string, *busapi.UACInfo, int) (string, error)
	DecryptJWT(interface{}) (*UACClaims, error)
}

type JWTCrypto struct {
	JWTSecret string
}

var DefaultAuthTimeout = 15

func (jwtCrypto *JWTCrypto) EncryptJWT(uac string, uacInfo *busapi.UACInfo, authTimeout int) (string, error) {
	if authTimeout == 0 {
		authTimeout = DefaultAuthTimeout
	}

	claims := UACClaims{
		UAC:         uac,
		AuthTimeout: authTimeout,
		UACInfo: busapi.UACInfo{
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
		return nil, fmt.Errorf("no JWT token in session")
	}
	jwtTokenString, ok := jwtToken.(string)
	if !ok || jwtTokenString == "" {
		return nil, fmt.Errorf("invalid JWT token type in session")
	}

	token, err := jwt.ParseWithClaims(jwtTokenString, &UACClaims{}, func(token *jwt.Token) (interface{}, error) {
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
