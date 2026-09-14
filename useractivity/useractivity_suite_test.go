package useractivity_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUserActivity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "User Activity Suite")
}
