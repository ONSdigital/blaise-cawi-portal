package login

import (
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/golang-jwt/jwt"
)

type UACClaims struct {
	UAC string `json:"uac"`
	busapi.UacInfo
	jwt.StandardClaims
}

func (uacClaims *UACClaims) AuthenticatedForInstrument(instrumentName string) bool {
	return strings.ToLower(uacClaims.UacInfo.InstrumentName) == strings.ToLower(instrumentName)
}

func (uacClaims *UACClaims) AuthenticatedForCase(caseID string) bool {
	return strings.ToLower(uacClaims.UacInfo.CaseID) == strings.ToLower(caseID)
}
