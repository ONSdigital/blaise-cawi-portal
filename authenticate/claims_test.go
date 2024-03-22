package authenticate_test

import (
	"fmt"
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
		disabled       = false
		claim          = &authenticate.UACClaims{
			UAC:         "0008901",
			AuthTimeout: 15,
			UacInfo: busapi.UacInfo{
				InstrumentName: instrumentName,
				CaseID:         caseID,
				Disabled:       disabled,
			},
		}
	)

	DescribeTable("CheckDiaInstrument",
		func(instrumentName1, instrumentName2 string, expected bool) {
			// Use the checkDiaInstrument function from the imported package
			Expect(claim.CheckDiaInstrument(instrumentName1, instrumentName2)).To(Equal(expected))
		},
		Entry("both diaA and diaB", "dia1234a", "dia1234b", true),
		Entry("only diaA", "dia1234a", "notdia5678b", false),
		Entry("only diaB", "notdia1234a", "dia5678b", false),
		Entry("neither diaA nor diaB", "notdia1234a", "notdia5678b", false),
		Entry("different case for diaA", "Dia1234a", "dia5678b", false),
		Entry("different case for diaB", "dia1234a", "DIA5678B", false),
		Entry("both in different case", "DIA1234A", "DIA5678B", false),
		Entry("invalid names", "bacon", "ham", false),
	)

	DescribeTable("AuthenticateForInstrument",
		func(testInstrumentName string, expected bool) {
			Expect(claim.AuthenticatedForInstrument(testInstrumentName)).To(Equal(expected))
		},
		Entry("same case", instrumentName, true),
		Entry("different case", strings.ToUpper(instrumentName), true),
	)

	DescribeTable("AuthenticateForCase",
		func(testCaseID string, disabled, expected bool) {
			  fmt.Println("Test Case ID:", testCaseID)
        fmt.Println("Case ID:", caseID)

			Expect(claim.AuthenticatedForCase(testCaseID)).To(Equal(expected))
		},
		Entry("same case", caseID, false, true),
		Entry("different case", strings.ToUpper(caseID), false, true),
		Entry("not matching", "bacon", false, false),
		Entry("is not authenticated when UAC is disabled", caseID, true, false),
	)

	Describe("LogFields", func() {
		It("Returns the instrument name and case ID as log fields", func() {
			fields := claim.LogFields()
			Expect(fields[0].String).To(Equal(instrumentName))
			Expect(fields[0].Key).To(Equal("AuthedInstrumentName"))
			Expect(fields[1].String).To(Equal(caseID))
			Expect(fields[1].Key).To(Equal("AuthedCaseID"))
			Expect(fields[2].Integer).To(Equal(int64(15)))
			Expect(fields[2].Key).To(Equal("AuthTimeout"))
		})
	})
})
