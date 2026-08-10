package authenticate_test

import (
	"strings"
	"testing"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
)

func newClaim() *authenticate.UACClaims {
	return &authenticate.UACClaims{
		UAC:         "0008901",
		AuthTimeout: 15,
		UACInfo: busapi.UACInfo{
			InstrumentName: "foo",
			CaseID:         "bar",
			Disabled:       false,
		},
	}
}

func TestAuthenticatedForInstrumentDiaPattern(t *testing.T) {
	claim := newClaim()

	tests := []struct {
		name               string
		authInstrumentName string
		testInstrumentName string
		expected           bool
	}{
		{name: "both diaA and diaB", authInstrumentName: "dia1234a", testInstrumentName: "dia1234b", expected: true},
		{name: "only diaA", authInstrumentName: "dia1234a", testInstrumentName: "notdia5678b", expected: false},
		{name: "only diaB", authInstrumentName: "notdia1234a", testInstrumentName: "dia5678b", expected: false},
		{name: "neither diaA nor diaB", authInstrumentName: "notdia1234a", testInstrumentName: "notdia5678b", expected: false},
		{name: "different case for diaA", authInstrumentName: "Dia1234a", testInstrumentName: "dia5678b", expected: false},
		{name: "different case for diaB", authInstrumentName: "dia1234a", testInstrumentName: "DIA5678B", expected: false},
		{name: "both in different case", authInstrumentName: "DIA1234A", testInstrumentName: "DIA5678B", expected: false},
		{name: "invalid names", authInstrumentName: "bacon", testInstrumentName: "ham", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claim.InstrumentName = tt.authInstrumentName
			if got := claim.AuthenticatedForInstrument(tt.testInstrumentName); got != tt.expected {
				t.Fatalf("AuthenticatedForInstrument(%q) with claim.InstrumentName=%q = %v, want %v", tt.testInstrumentName, tt.authInstrumentName, got, tt.expected)
			}
		})
	}
}

func TestAuthenticatedForInstrument(t *testing.T) {
	claim := newClaim()
	instrumentName := claim.InstrumentName

	tests := []struct {
		name               string
		testInstrumentName string
		expected           bool
	}{
		{name: "same case", testInstrumentName: instrumentName, expected: true},
		{name: "different case", testInstrumentName: strings.ToUpper(instrumentName), expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := claim.AuthenticatedForInstrument(tt.testInstrumentName); got != tt.expected {
				t.Fatalf("AuthenticatedForInstrument(%q) = %v, want %v", tt.testInstrumentName, got, tt.expected)
			}
		})
	}
}

func TestAuthenticatedForCase(t *testing.T) {
	claim := newClaim()
	caseID := claim.CaseID

	tests := []struct {
		name      string
		testCaseID string
		disabled  bool
		expected  bool
	}{
		{name: "same case", testCaseID: caseID, disabled: false, expected: true},
		{name: "different case", testCaseID: strings.ToUpper(caseID), disabled: false, expected: true},
		{name: "not matching", testCaseID: "bacon", disabled: false, expected: false},
		{name: "is not authenticated when UAC is disabled", testCaseID: caseID, disabled: true, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claim.Disabled = tt.disabled
			if got := claim.AuthenticatedForCase(tt.testCaseID); got != tt.expected {
				t.Fatalf("AuthenticatedForCase(%q) = %v, want %v", tt.testCaseID, got, tt.expected)
			}
		})
	}
}

func TestLogFields(t *testing.T) {
	claim := newClaim()
	fields := claim.LogFields()

	if len(fields) < 3 {
		t.Fatalf("len(fields) = %d, want at least 3", len(fields))
	}
	if fields[0].String != claim.InstrumentName || fields[0].Key != "AuthedInstrumentName" {
		t.Fatalf("fields[0] = %+v, want AuthedInstrumentName=%s", fields[0], claim.InstrumentName)
	}
	if fields[1].String != "fcde2b2edba5" || fields[1].Key != "AuthedCaseIDFingerprint" {
		t.Fatalf("fields[1] = %+v, want AuthedCaseIDFingerprint=fcde2b2edba5", fields[1])
	}
	if fields[2].Integer != int64(claim.AuthTimeout) || fields[2].Key != "AuthTimeout" {
		t.Fatalf("fields[2] = %+v, want AuthTimeout=%d", fields[2], claim.AuthTimeout)
	}
}
