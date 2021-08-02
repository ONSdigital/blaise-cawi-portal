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
		caseID         = "fwibble"
		postcode       = "NP10 8XG"
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

	Describe("Get Post code", func() {
		Context("when an case does not exist", func() {
			JustBeforeEach(func() {
				httpmock.RegisterResponder("GET", fmt.Sprintf("%s/api/v1/serverparks/%s/instruments/%s/cases/%s/postcode", restApiUrl, serverpark, instrumentName, caseID),
					httpmock.NewBytesResponder(404, []byte{}))
			})

			It("returns a NotFound error", func() {
				postcodeRespose, err := blaiseRestApi.GetPostCode(instrumentName, caseID)
				Expect(err).To(MatchError("case not found"))
				Expect(postcodeRespose).To(BeEmpty())
			})
		})

		Context("when there is a postcode", func() {
			JustBeforeEach(func() {
				httpmock.RegisterResponder("GET", fmt.Sprintf("%s/api/v1/serverparks/%s/instruments/%s/cases/%s/postcode", restApiUrl, serverpark, instrumentName, caseID),
					httpmock.NewJsonResponderOrPanic(200, postcode))
			})

			It("returns a populated postcode", func() {
				postcodeRespose, err := blaiseRestApi.GetPostCode(instrumentName, caseID)
				Expect(err).To(BeNil())
				Expect(postcodeRespose).To(Equal(postcode))
			})
		})

		Context("when there is no postcode", func() {
			JustBeforeEach(func() {
				httpmock.RegisterResponder("GET", fmt.Sprintf("%s/api/v1/serverparks/%s/instruments/%s/cases/%s/postcode", restApiUrl, serverpark, instrumentName, caseID),
					httpmock.NewJsonResponderOrPanic(200, ""))
			})

			It("returns an empty postcode", func() {
				postcodeRespose, err := blaiseRestApi.GetPostCode(instrumentName, caseID)
				Expect(err).To(BeNil())
				Expect(postcodeRespose).To(BeEmpty())
			})
		})
	})
})
