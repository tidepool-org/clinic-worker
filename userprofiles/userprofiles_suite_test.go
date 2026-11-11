package userprofiles_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUserProfiles(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "User Profiles Suite")
}
