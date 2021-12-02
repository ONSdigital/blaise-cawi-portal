package blaiserestapi_test

import (
	"fmt"
	"net/http"

	"github.com/ONSdigital/blaise-cawi-portal/blaiserestapi"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Blaise rest api endpoints", func() {
	var (
		restApiUrl     = "http://localhost"
		serverpark     = "foobar"
		instrumentName = "lolcats"
		blaiseRestApi  = &blaiserestapi.BlaiseRestApi{
			BaseUrl:    restApiUrl,
			Serverpark: serverpark,
			Client:     &http.Client{},
		}
	)

	BeforeEach(func() {
		httpmock.Activate()
	})

	AfterEach(func() {
		httpmock.DeactivateAndReset()
	})

	Describe("Get instrument settings", func() {
		Context("when the instrument does not exist", func() {
			JustBeforeEach(func() {
				httpmock.RegisterResponder("GET", fmt.Sprintf("%s/api/v1/serverparks/%s/instruments/%s/settings", restApiUrl, serverpark, instrumentName),
					httpmock.NewBytesResponder(404, []byte{}))
			})

			It("returns a NotFound error", func() {
				instrumentSettings, err := blaiseRestApi.GetInstrumentSettings(instrumentName)
				Expect(err).To(MatchError("instrument not found"))
				Expect(instrumentSettings).To(BeEmpty())
			})
		})

		Context("when the instrument does exist", func() {
			JustBeforeEach(func() {
				httpmock.RegisterResponder("GET", fmt.Sprintf("%s/api/v1/serverparks/%s/instruments/%s/settings", restApiUrl, serverpark, instrumentName),
					httpmock.NewJsonResponderOrPanic(200, blaiserestapi.InstrumentSettings{
						{
							Type:           "StrictInterviewing",
							SessionTimeout: 15,
						},
					}))
			})

			It("returns instrument settings", func() {
				instrumentSettings, err := blaiseRestApi.GetInstrumentSettings(instrumentName)
				Expect(err).To(BeNil())
				Expect(instrumentSettings).To(HaveLen(1))
				Expect(instrumentSettings[0].Type).To(Equal("StrictInterviewing"))
				Expect(instrumentSettings[0].SessionTimeout).To(Equal(15))
			})
		})
	})
})

var _ = Describe("InstrumentSettings.StrictInterviewing", func() {
	Context("when the instrument settings include a 'StrictInterviewing' type", func() {
		It("returns the StrictInterviewing settings block", func() {
			var instrumentSettings = blaiserestapi.InstrumentSettings{
				{
					Type:                 "StrictInterviewing",
					SessionTimeout:       15,
					SaveSessionOnTimeout: true,
				},
				{
					Type:           "StrictCati",
					SessionTimeout: 55,
				},
				{
					Type:                 "StrictInterviewing",
					SessionTimeout:       56,
					SaveSessionOnTimeout: false,
				},
			}
			Expect(instrumentSettings.StrictInterviewing().SessionTimeout).To(Equal(15))
			Expect(instrumentSettings.StrictInterviewing().Type).To(Equal("StrictInterviewing"))
		})
	})

	Context("when the instrument settings do not include a 'StrictInterviewing' type", func() {
		It("returns an empty settings block", func() {
			var instrumentSettings = blaiserestapi.InstrumentSettings{
				{
					Type:                 "FreeInterviewing",
					SessionTimeout:       15,
					SaveSessionOnTimeout: true,
				},
				{
					Type:           "StrictCati",
					SessionTimeout: 15,
				},
			}
			Expect(instrumentSettings.StrictInterviewing()).To(Equal(blaiserestapi.InstrumentSettingsType{}))
		})
	})
})
