package authenticate

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

var (
	diaInstrumentPatternA = regexp.MustCompile(`^dia\d{4}a$`)
	diaInstrumentPatternB = regexp.MustCompile(`^dia\d{4}b$`)
)

type UACClaims struct {
	UAC         string `json:"uac"`
	AuthTimeout int    `json:"auth_timeout"`
	busapi.UACInfo
	jwt.RegisteredClaims
}

func (uacClaims *UACClaims) AuthenticatedForInstrument(instrumentName string) bool {
	return strings.EqualFold(uacClaims.UACInfo.InstrumentName, instrumentName) ||
		uacClaims.checkDiaInstrument(uacClaims.UACInfo.InstrumentName, instrumentName)
}

func (uacClaims *UACClaims) checkDiaInstrument(instrumentName1, instrumentName2 string) bool {
	return diaInstrumentPatternA.MatchString(instrumentName1) &&
		diaInstrumentPatternB.MatchString(instrumentName2)
}

func (uacClaims *UACClaims) AuthenticatedForCase(caseID string) bool {
	if !uacClaims.UACInfo.Disabled {
		return strings.EqualFold(uacClaims.UACInfo.CaseID, caseID)
	}
	return false
}

func (uacClaims *UACClaims) LogFields() []zap.Field {
	fields := make([]zap.Field, 0, 3)
	fields = append(fields, zap.String("AuthedInstrumentName", uacClaims.UACInfo.InstrumentName))
	fields = append(fields, zap.String("AuthedCaseIDFingerprint", CaseIDFingerprint(uacClaims.UACInfo.CaseID)))
	fields = append(fields, zap.Int("AuthTimeout", uacClaims.AuthTimeout))
	return fields
}

func CaseIDFingerprint(caseID string) string {
	normalised := strings.TrimSpace(strings.ToLower(caseID))
	if normalised == "" {
		return "unknown"
	}
	hash := sha256.Sum256([]byte(normalised))
	return hex.EncodeToString(hash[:])[:12]
}
