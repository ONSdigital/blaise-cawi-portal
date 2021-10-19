package authenticate

import (
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/golang-jwt/jwt"
	"go.uber.org/zap"
)

type UACClaims struct {
	UAC string `json:"uac"`
	busapi.UacInfo
	jwt.StandardClaims
}

func (uacClaims *UACClaims) AuthenticatedForInstrument(instrumentName string) bool {
	return strings.EqualFold(uacClaims.UacInfo.InstrumentName, instrumentName)
}

func (uacClaims *UACClaims) AuthenticatedForCase(caseID string) bool {
	return strings.EqualFold(uacClaims.UacInfo.CaseID, caseID)
}

func (uacClaims *UACClaims) LogFields() []zap.Field {
	var fields []zap.Field
	fields = append(fields, zap.String("AuthedInstrumentName", uacClaims.UacInfo.InstrumentName))
	fields = append(fields, zap.String("AuthedCaseID", uacClaims.UacInfo.CaseID))
	return fields
}
