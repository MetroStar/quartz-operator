/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/MetroStar/quartz-operator/internal/testutil"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	// These variables are maintained for backward compatibility
	ctx       context.Context
	cancel    context.CancelFunc
	testEnv   *envtest.Environment
	cfg       *rest.Config
	k8sClient client.Client

	// The shared test environment instance
	sharedTestEnv *testutil.TestEnv
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	// Setup the shared test environment
	sharedTestEnv = testutil.SetupTestEnv()

	// Set the backward compatibility variables
	ctx = sharedTestEnv.Ctx
	cancel = sharedTestEnv.Cancel
	testEnv = sharedTestEnv.TestEnv
	cfg = sharedTestEnv.Cfg
	k8sClient = sharedTestEnv.K8sClient
})

var _ = AfterSuite(func() {
	// Teardown the shared test environment
	sharedTestEnv.TeardownTestEnv()
})
