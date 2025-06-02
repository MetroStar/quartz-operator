package services

import (
	"context"
	"strings"

	"github.com/brianvoe/gofakeit/v7"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("LookupService", func() {
	var (
		ctx           context.Context
		c             client.Client
		lookupService *LookupService
	)

	BeforeEach(func() {
		ctx = context.Background()
		c = testEnv.K8sClient

		// Initialize the LookupService with a test config
		lookupService = NewLookupService(c, testEnv.Cfg)
	})

	Describe("LookupGroupKind", func() {
		It("should find known kinds", func() {
			gvk, err := lookupService.LookupGroupKind("Pod")
			Expect(err).NotTo(HaveOccurred())
			Expect(gvk.Kind).To(Equal("Pod"))
			Expect(gvk.Group).To(Equal(""))
			Expect(gvk.Version).To(Equal("v1"))

			gvk, err = lookupService.LookupGroupKind("Deployment")
			Expect(err).NotTo(HaveOccurred())
			Expect(gvk.Kind).To(Equal("Deployment"))
			Expect(gvk.Group).To(Equal("apps"))
			Expect(gvk.Version).To(Equal("v1"))
		})

		It("should handle kind.group notation", func() {
			gvk, err := lookupService.LookupGroupKind("Deployment.apps")
			Expect(err).NotTo(HaveOccurred())
			Expect(gvk.Kind).To(Equal("Deployment"))
			Expect(gvk.Group).To(Equal("apps"))
			Expect(gvk.Version).To(Equal("v1"))
		})

		It("should return error for unknown kinds", func() {
			_, err := lookupService.LookupGroupKind("UnknownKind")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ListResources", func() {
		It("should list resources of a specific kind in a namespace", func() {

			suffix := strings.ToLower(gofakeit.Word())
			ns := fakeNamespace("lookupservice-" + suffix)
			pod1 := fakePod("test-pod-1-"+suffix, ns.GetName())
			pod2 := fakePod("test-pod-2-"+suffix, ns.GetName())

			// Add pods to the fake client
			Expect(c.Create(ctx, ns)).To(Succeed())
			Expect(c.Create(ctx, pod1)).To(Succeed())
			Expect(c.Create(ctx, pod2)).To(Succeed())

			// Test ListResources function
			gvk := schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			}

			list, err := lookupService.ListResources(ctx, gvk, ns.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Items).To(HaveLen(2))

			names := []string{}
			for _, item := range list.Items {
				names = append(names, item.GetName())
			}
			Expect(names).To(ContainElements(pod1.GetName(), pod2.GetName()))
		})
	})
})
