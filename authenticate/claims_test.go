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

	Describe("LogFields", func() {
		It("Returns the instrument name and case ID as log fields", func() {
			fields := claim.LogFields()
			Expect(fields[0].String).To(Equal(instrumentName))
			Expect(fields[0].Key).To(Equal("AuthedInstrumentName"))
			Expect(fields[1].String).To(Equal(caseID))
			Expect(fields[1].Key).To(Equal("AuthedCaseID"))
		})
	})
})
