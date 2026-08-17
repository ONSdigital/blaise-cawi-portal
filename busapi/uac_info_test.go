package busapi_test

import (
	"testing"

	"github.com/ONSdigital/blaise-cawi-portal/busapi"
)

func TestUacInfoInvalidCase(t *testing.T) {
	tests := []struct {
		name           string
		instrumentName string
		caseID         string
		expected       bool
	}{
		{name: "no caseID", instrumentName: "instrumentFoo", caseID: "", expected: true},
		{name: "no instrumentName", instrumentName: "", caseID: "caseFoo", expected: true},
		{name: "unknown caseID", instrumentName: "instrumentFoo", caseID: "unknown", expected: true},
		{name: "unknown instrumentName", instrumentName: "unknown", caseID: "caseFoo", expected: true},
		{name: "valid", instrumentName: "instrumentFoo", caseID: "caseFoo", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uacInfo := busapi.UACInfo{
				InstrumentName: tt.instrumentName,
				CaseID:         tt.caseID,
			}

			if got := uacInfo.InvalidCase(); got != tt.expected {
				t.Fatalf("InvalidCase() = %v, want %v", got, tt.expected)
			}
		})
	}
}
