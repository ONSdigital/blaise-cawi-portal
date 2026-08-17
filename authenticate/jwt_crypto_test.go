package authenticate_test

import (
	"testing"
	"time"

	"github.com/ONSdigital/blaise-cawi-portal/authenticate"
	"github.com/ONSdigital/blaise-cawi-portal/busapi"
)

func TestJWTCryptoEncryptJWT(t *testing.T) {
	jwtCrypto := &authenticate.JWTCrypto{JWTSecret: "test-secret"}

	t.Run("sets expiry to roughly authTimeout minutes", func(t *testing.T) {
		authTimeout := 7
		before := time.Now()
		tolerance := 2 * time.Second

		token, err := jwtCrypto.EncryptJWT("123456789012", &busapi.UACInfo{
			InstrumentName: "foo",
			CaseID:         "bar",
		}, authTimeout)
		if err != nil {
			t.Fatalf("EncryptJWT() error: %v", err)
		}

		claims, err := jwtCrypto.DecryptJWT(token)
		if err != nil {
			t.Fatalf("DecryptJWT() error: %v", err)
		}
		if claims == nil {
			t.Fatal("DecryptJWT() claims is nil")
		}

		after := time.Now()
		expiresAt := claims.ExpiresAt.Time
		min := before.Add(time.Duration(authTimeout)*time.Minute - tolerance)
		max := after.Add(time.Duration(authTimeout)*time.Minute + tolerance)
		if expiresAt.Before(min) || expiresAt.After(max) {
			t.Fatalf("expiresAt %v out of range [%v, %v]", expiresAt, min, max)
		}
		if claims.Issuer != authenticate.ISSUER {
			t.Fatalf("Issuer = %q, want %q", claims.Issuer, authenticate.ISSUER)
		}
	})

	t.Run("uses the default timeout when authTimeout is zero", func(t *testing.T) {
		before := time.Now()
		tolerance := 2 * time.Second

		token, err := jwtCrypto.EncryptJWT("123456789012", &busapi.UACInfo{
			InstrumentName: "foo",
			CaseID:         "bar",
		}, 0)
		if err != nil {
			t.Fatalf("EncryptJWT() error: %v", err)
		}

		claims, err := jwtCrypto.DecryptJWT(token)
		if err != nil {
			t.Fatalf("DecryptJWT() error: %v", err)
		}
		if claims == nil {
			t.Fatal("DecryptJWT() claims is nil")
		}

		after := time.Now()
		expected := time.Duration(authenticate.DefaultAuthTimeout) * time.Minute
		expiresAt := claims.ExpiresAt.Time
		min := before.Add(expected - tolerance)
		max := after.Add(expected + tolerance)
		if expiresAt.Before(min) || expiresAt.After(max) {
			t.Fatalf("expiresAt %v out of range [%v, %v]", expiresAt, min, max)
		}
		if claims.AuthTimeout != authenticate.DefaultAuthTimeout {
			t.Fatalf("AuthTimeout = %d, want %d", claims.AuthTimeout, authenticate.DefaultAuthTimeout)
		}
		if claims.Issuer != authenticate.ISSUER {
			t.Fatalf("Issuer = %q, want %q", claims.Issuer, authenticate.ISSUER)
		}
	})
}

func TestJWTCryptoDecryptJWTErrors(t *testing.T) {
	jwtCrypto := &authenticate.JWTCrypto{JWTSecret: "test-secret"}

	t.Run("returns an error when jwt token type is not a string", func(t *testing.T) {
		claims, err := jwtCrypto.DecryptJWT(123)
		if err == nil {
			t.Fatal("DecryptJWT() expected error, got nil")
		}
		if err.Error() != "invalid JWT token type in session" {
			t.Fatalf("error = %q, want %q", err.Error(), "invalid JWT token type in session")
		}
		if claims != nil {
			t.Fatalf("claims = %+v, want nil", claims)
		}
	})

	t.Run("returns an error when jwt token is nil", func(t *testing.T) {
		claims, err := jwtCrypto.DecryptJWT(nil)
		if err == nil {
			t.Fatal("DecryptJWT() expected error, got nil")
		}
		if err.Error() != "no JWT token in session" {
			t.Fatalf("error = %q, want %q", err.Error(), "no JWT token in session")
		}
		if claims != nil {
			t.Fatalf("claims = %+v, want nil", claims)
		}
	})
}
