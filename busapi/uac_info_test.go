package busapi_test

import (
	"time"

	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParsePostcodeAttemptTimestamp", func() {
	Context("when the timestamp is valid", func() {
		It("does not errror", func() {
			_, err := busapi.UacInfo{PostcodeAttemptTimestamp: time.Now().UTC().String()}.ParsePostcodeAttemptTimestamp()
			Expect(err).To(BeNil())
		})
	})

	Context("when the timestamp is invalid", func() {
		It("returns an error", func() {
			_, err := busapi.UacInfo{PostcodeAttemptTimestamp: "not_a_time"}.ParsePostcodeAttemptTimestamp()
			Expect(err).To(MatchError(`parsing time "not_a_time" as "2006-01-02 15:04:05.999999999 -0700 MST": cannot parse "not_a_time" as "2006"`))
		})
	})
})

var _ = Describe("PostcodeAttemptsExpired", func() {
	Context("then the attempts have expired", func() {
		It("returns true", func() {
			uacInfo := busapi.UacInfo{PostcodeAttemptTimestamp: time.Now().UTC().Add(-45 * time.Minute).String()}
			expired, err := uacInfo.PostcodeAttemptsExpired()
			Expect(err).To(BeNil())
			Expect(expired).To(BeTrue())
		})
	})

	Context("when the attempts have not expired", func() {
		It("returns false", func() {
			uacInfo := busapi.UacInfo{PostcodeAttemptTimestamp: time.Now().UTC().String()}
			expired, err := uacInfo.PostcodeAttemptsExpired()
			Expect(err).To(BeNil())
			Expect(expired).To(BeFalse())
		})
	})
})

var _ = Describe("TooManyUnexpiredAttempts", func() {
	Context("when there are under 5 attempts", func() {
		It("returns false", func() {
			uacInfo := busapi.UacInfo{
				PostcodeAttemptTimestamp: time.Now().UTC().String(),
				PostcodeAttempts:         2,
			}
			expired, err := uacInfo.TooManyUnexpiredAttempts()
			Expect(err).To(BeNil())
			Expect(expired).To(BeFalse())
		})
	})

	Context("when there are more than 5 attempts", func() {
		Context("and the attempts have expired", func() {
			It("returns false", func() {
				uacInfo := busapi.UacInfo{
					PostcodeAttemptTimestamp: time.Now().UTC().Add(-45 * time.Minute).String(),
					PostcodeAttempts:         6,
				}
				expired, err := uacInfo.TooManyUnexpiredAttempts()
				Expect(err).To(BeNil())
				Expect(expired).To(BeFalse())
			})
		})

		Context("and the attempts have not expired", func() {
			It("returns true", func() {
				uacInfo := busapi.UacInfo{
					PostcodeAttemptTimestamp: time.Now().UTC().String(),
					PostcodeAttempts:         6,
				}
				expired, err := uacInfo.TooManyUnexpiredAttempts()
				Expect(err).To(BeNil())
				Expect(expired).To(BeTrue())
			})
		})
	})
})

var _ = DescribeTable("TooManyAttempts",
	func(attempts int, expected bool) {
		uacInfo := busapi.UacInfo{
			PostcodeAttempts: attempts,
		}
		Expect(uacInfo.TooManyAttempts()).To(Equal(expected))
	},
	Entry("1", 1, false),
	Entry("0", 0, false),
	Entry("4", 4, false),
	Entry("5", 5, true),
	Entry("6", 6, true),
	Entry("31236", 31236, true),
)
