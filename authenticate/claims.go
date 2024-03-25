package authenticate

import (
	"fmt"
	"regexp"
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
	return uacClaims.CheckDiaInstrument(uacClaims.UacInfo.InstrumentName, instrumentName)
}

func (uacClaims *UACClaims) CheckDiaInstrument(instrumentName1, instrumentName2 string) bool {
	diaA, diaAErr := regexp.MatchString(`^dia\d{4}a$`, instrumentName1)
	if diaAErr != nil {
		fmt.Print(diaAErr)
	}

	diaB, diaBErr := regexp.MatchString(`^dia\d{4}b$`, instrumentName2)
	if diaBErr != nil {
		fmt.Print(diaBErr)
	}

	if diaA && diaB {
		return true
	}
	return false
}


func (uacClaims *UACClaims) AuthenticatedForCase(caseID string) bool {
	if !uacClaims.UacInfo.Disabled  {
        return strings.EqualFold(uacClaims.UacInfo.CaseID, caseID)
    }
    return false
}

func (uacClaims *UACClaims) LogFields() []zap.Field {
	var fields []zap.Field
	fields = append(fields, zap.String("AuthedInstrumentName", uacClaims.UacInfo.InstrumentName))
	fields = append(fields, zap.String("AuthedCaseID", uacClaims.UacInfo.CaseID))
	fields = append(fields, zap.Int("AuthTimeout", uacClaims.AuthTimeout))
	return fields
}
