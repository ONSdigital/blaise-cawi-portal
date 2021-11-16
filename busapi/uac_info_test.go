package busapi_test

import (
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = DescribeTable("InvalidCase",
	func(instrumentName, caseID string, expected bool) {
		uacInfo := busapi.UacInfo{
			InstrumentName: instrumentName,
			CaseID:         caseID,
		}
		Expect(uacInfo.InvalidCase()).To(Equal(expected))
	},
	Entry("No caseID", "instrumentFoo", "", true),
	Entry("No insturmentName", "", "caseFoo", true),
	Entry("Unknown caseID", "instrumentFoo", "unknown", true),
	Entry("Unknown instrumentName", "unknown", "caseFoo", true),
	Entry("Valid", "instrumentFoo", "caseFoo", false),
)
