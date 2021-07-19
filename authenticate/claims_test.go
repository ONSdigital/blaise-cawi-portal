package authenticate_test

import (
	"strings"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Claims", func() {

	var (
		instrumentName = "foo"
		caseID         = "bar"
		claim          = &authenticate.UACClaims{
			UAC: "0008901",
			UacInfo: busapi.UacInfo{
				InstrumentName: instrumentName,
				CaseID:         caseID,
			},
		}
	)

	DescribeTable("AuthenticateForInstrument",
		func(testInstrumentName string, expected bool) {
			Expect(claim.AuthenticatedForInstrument(testInstrumentName)).To(Equal(expected))
		},
		Entry("same case", instrumentName, true),
		Entry("different case", strings.ToUpper(instrumentName), true),
		Entry("not matching", "bacon", false),
	)

	DescribeTable("AuthenticateForCase",
		func(testCaseID string, expected bool) {
			Expect(claim.AuthenticatedForCase(testCaseID)).To(Equal(expected))
		},
		Entry("same case", caseID, true),
		Entry("different case", strings.ToUpper(caseID), true),
		Entry("not matching", "bacon", false),
	)
})
