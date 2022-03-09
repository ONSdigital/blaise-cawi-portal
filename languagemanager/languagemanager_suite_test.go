package languagemanager_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLanguagemanager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Languagemanager Suite")
}
