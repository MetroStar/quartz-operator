package services

import (
	"context"
	"strings"

	"github.com/brianvoe/gofakeit/v7"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cleanupv1alpha1 "github.com/MetroStar/quartz-operator/api/v1alpha1"
)

var _ = Describe("DeleteService", func() {
	var (
		ctx           context.Context
		c             client.Client
		deleteService *DeleteService
		lookupService *LookupService
		ns            *corev1.Namespace
		pod1          *corev1.Pod
		pod2          *corev1.Pod
	)

	BeforeEach(func() {
		ctx = context.Background()
		c = testEnv.K8sClient

		// Create test resources
		suffix := strings.ToLower(gofakeit.Word())
		ns = fakeNamespace("deleteservice-" + suffix)
		pod1 = fakePod("test-pod-1-"+suffix, ns.GetName())
		pod2 = fakePod("test-pod-2-"+suffix, ns.GetName())

		Expect(c.Create(ctx, ns)).To(Succeed())
		Expect(c.Create(ctx, pod1)).To(Succeed())
		Expect(c.Create(ctx, pod2)).To(Succeed())

		// Initialize services
		lookupService = NewLookupService(c, testEnv.Cfg)
		deleteService = NewDeleteService(ctx, c, lookupService)
	})

	Describe("DeleteNamedResource", func() {
		It("should delete a specific resource by name", func() {
			// Test DeleteNamedResource function
			gvk := schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			}

			count, err := deleteService.DeleteNamedResource(ctx, false, gvk, ns.GetName(), pod1.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Verify the resource was deleted
			err = c.Get(ctx, types.NamespacedName{Namespace: ns.GetName(), Name: pod1.GetName()},
				&metav1.PartialObjectMetadata{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}})
			Expect(err).To(HaveOccurred())
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			// Verify the other resource still exists
			err = c.Get(ctx, types.NamespacedName{Namespace: ns.GetName(), Name: pod2.GetName()},
				&metav1.PartialObjectMetadata{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not actually delete in dry run mode", func() {
			gvk := schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			}

			count, err := deleteService.DeleteNamedResource(ctx, true, gvk, ns.GetName(), pod1.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Verify the resource still exists
			err = c.Get(ctx, types.NamespacedName{Namespace: ns.GetName(), Name: pod1.GetName()},
				&metav1.PartialObjectMetadata{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error for non-existent resources", func() {
			gvk := schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			}

			_, err := deleteService.DeleteNamedResource(ctx, false, gvk, ns.GetName(), "non-existent-pod")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("DeleteResources", func() {
		It("should delete all resources of a specific kind in a namespace", func() {
			gvk := schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			}

			count, err := deleteService.DeleteResources(ctx, false, gvk, ns.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(2))

			// Verify all resources were deleted
			list := &metav1.PartialObjectMetadataList{}
			list.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "PodList",
			})

			err = c.List(ctx, list, client.InNamespace(ns.GetName()))
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Items).To(HaveLen(0))
		})

		It("should not actually delete in dry run mode", func() {
			gvk := schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			}

			count, err := deleteService.DeleteResources(ctx, true, gvk, ns.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(2))

			// Verify resources still exist
			list := &metav1.PartialObjectMetadataList{}
			list.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "PodList",
			})

			err = c.List(ctx, list, client.InNamespace(ns.GetName()))
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Items).To(HaveLen(2))
		})
	})

	Describe("DeleteItem", func() {
		It("should delete a specific item by name", func() {
			item := cleanupv1alpha1.PreClusterDestroyCleanupItem{
				Kind:      "Pod",
				Namespace: ns.GetName(),
				Name:      pod1.GetName(),
				Action:    cleanupv1alpha1.ActionDelete,
			}

			count, err := deleteService.DeleteItem(ctx, false, schema.GroupVersionKind{Kind: "Pod", Version: "v1"}, item)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Verify the resource was deleted
			err = c.Get(ctx, types.NamespacedName{Namespace: ns.GetName(), Name: pod1.GetName()},
				&metav1.PartialObjectMetadata{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}})
			Expect(err).To(HaveOccurred())
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		It("should delete all items of a kind when no name is specified", func() {
			item := cleanupv1alpha1.PreClusterDestroyCleanupItem{
				Kind:      "Pod",
				Namespace: ns.GetName(),
				Action:    cleanupv1alpha1.ActionDelete,
			}

			count, err := deleteService.DeleteItem(ctx, false, schema.GroupVersionKind{Kind: "Pod", Version: "v1"}, item)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(2))

			// Verify all resources were deleted
			list := &metav1.PartialObjectMetadataList{}
			list.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "PodList",
			})

			err = c.List(ctx, list, client.InNamespace(ns.GetName()))
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Items).To(HaveLen(0))
		})
	})
})
