package services

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/MetroStar/quartz-operator/internal/testutil"
)

var testEnv *testutil.TestEnv

func TestServices(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Services Suite")
}

var _ = BeforeSuite(func() {
	// Setup the shared test environment
	testEnv = testutil.SetupTestEnv()
})

var _ = AfterSuite(func() {
	// Teardown the shared test environment
	testEnv.TeardownTestEnv()
})
