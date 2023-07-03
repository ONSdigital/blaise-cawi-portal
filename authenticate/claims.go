package authenticate

import (
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/golang-jwt/jwt"
	"go.uber.org/zap"
)

type UACClaims struct {
	UAC         string `json:"uac"`
	AuthTimeout int    `json:"auth_timeout"`
	busapi.UacInfo
	jwt.StandardClaims
}

func (uacClaims *UACClaims) AuthenticatedForInstrument(instrumentName string) bool {
	if strings.EqualFold(uacClaims.UacInfo.InstrumentName, instrumentName) {
		return true
	}
	if uacClaims.UacInfo.InstrumentName == "dia2299a" && instrumentName == "dia2299b" {
		return true
	}
	return false
}

func (uacClaims *UACClaims) AuthenticatedForCase(caseID string) bool {
	return strings.EqualFold(uacClaims.UacInfo.CaseID, caseID)
}

func (uacClaims *UACClaims) LogFields() []zap.Field {
	var fields []zap.Field
	fields = append(fields, zap.String("AuthedInstrumentName", uacClaims.UacInfo.InstrumentName))
	fields = append(fields, zap.String("AuthedCaseID", uacClaims.UacInfo.CaseID))
	fields = append(fields, zap.Int("AuthTimeout", uacClaims.AuthTimeout))
	return fields
}
