package authenticate_test

import (
	"time"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("JWTCrypto", func() {
	var jwtCrypto *authenticate.JWTCrypto

	BeforeEach(func() {
		jwtCrypto = &authenticate.JWTCrypto{JWTSecret: "test-secret"}
	})

	It("sets expiry to roughly authTimeout minutes", func() {
		authTimeout := 7
		before := time.Now()
		tolerance := 2 * time.Second

		token, err := jwtCrypto.EncryptJWT("123456789012", &busapi.UacInfo{
			InstrumentName: "foo",
			CaseID:         "bar",
		}, authTimeout)
		Expect(err).To(BeNil())

		claims, err := jwtCrypto.DecryptJWT(token)
		Expect(err).To(BeNil())
		Expect(claims).ToNot(BeNil())

		after := time.Now()
		expiresAt := claims.ExpiresAt.Time

		Expect(expiresAt).To(BeTemporally(">=", before.Add(time.Duration(authTimeout)*time.Minute-tolerance)))
		Expect(expiresAt).To(BeTemporally("<=", after.Add(time.Duration(authTimeout)*time.Minute+tolerance)))
		Expect(claims.Issuer).To(Equal(authenticate.ISSUER))
	})

	It("uses the default timeout when authTimeout is zero", func() {
		before := time.Now()
		tolerance := 2 * time.Second

		token, err := jwtCrypto.EncryptJWT("123456789012", &busapi.UacInfo{
			InstrumentName: "foo",
			CaseID:         "bar",
		}, 0)
		Expect(err).To(BeNil())

		claims, err := jwtCrypto.DecryptJWT(token)
		Expect(err).To(BeNil())
		Expect(claims).ToNot(BeNil())

		after := time.Now()
		expected := time.Duration(authenticate.DefaultAuthTimeout) * time.Minute
		expiresAt := claims.ExpiresAt.Time

		Expect(expiresAt).To(BeTemporally(">=", before.Add(expected-tolerance)))
		Expect(expiresAt).To(BeTemporally("<=", after.Add(expected+tolerance)))
		Expect(claims.AuthTimeout).To(Equal(authenticate.DefaultAuthTimeout))
		Expect(claims.Issuer).To(Equal(authenticate.ISSUER))
	})
})