package clinicians_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestClinicians(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Clinicians Suite")
}
