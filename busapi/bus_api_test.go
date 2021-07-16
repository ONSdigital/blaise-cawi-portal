package busapi_test

import (
	"fmt"
	"net/http"

	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Getting a UAC from BUS", func() {
	var (
		baseUrl = "http://localhost"
		busApi  = &busapi.BusApi{
			BaseUrl: baseUrl,
			Client:  &http.Client{},
		}
		uac = "123456789012"
	)

	BeforeEach(func() {
		httpmock.Activate()
	})

	AfterEach(func() {
		httpmock.DeactivateAndReset()
	})

	Describe("Get UAC Info", func() {
		Context("when a uac is valid", func() {
			JustBeforeEach(func() {
				httpmock.RegisterResponder("POST", fmt.Sprintf("%s/uacs/uac", baseUrl),
					httpmock.NewJsonResponderOrPanic(200, busapi.UacInfo{InstrumentName: "foo", CaseID: "bar"}))
			})

			It("Returns UAC Info for a valid UAC", func() {
				uacInfo, err := busApi.GetUacInfo(uac)
				Expect(err).To(BeNil())
				Expect(uacInfo.InstrumentName).To(Equal("foo"))
				Expect(uacInfo.CaseID).To(Equal("bar"))
			})
		})

		Context("bad response is returned", func() {
			JustBeforeEach(func() {
				httpmock.RegisterResponder("POST", fmt.Sprintf("%s/uacs/uac", baseUrl),
					httpmock.NewJsonResponderOrPanic(500, "nil"))
			})

			It("Returns a an error and an empty uac info struct", func() {
				uacInfo, err := busApi.GetUacInfo(uac)
				Expect(err).To(MatchError("Unable To Unmarshal Json"))
				Expect(uacInfo.InstrumentName).To(Equal(""))
				Expect(uacInfo.CaseID).To(Equal(""))
			})
		})
	})
})
