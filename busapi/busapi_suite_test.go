package busapi_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBusapi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Busapi Suite")
}
