package services

import (
	"context"
	"strings"

	"github.com/brianvoe/gofakeit/v7"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cleanupv1alpha1 "github.com/MetroStar/quartz-operator/api/v1alpha1"
)

var _ = Describe("CleanupService", func() {
	var (
		ctx            context.Context
		c              client.Client
		cleanupService *CleanupService
		ns             *corev1.Namespace
		deployment     *appsv1.Deployment
		statefulSet    *appsv1.StatefulSet
	)

	BeforeEach(func() {
		ctx = context.Background()
		c = testEnv.K8sClient

		// Create test resources
		suffix := strings.ToLower(gofakeit.Word())
		ns = fakeNamespace("cleanupservice-" + suffix)
		deployment = fakeDeployment("test-deployment-"+suffix, ns.GetName())
		statefulSet = fakeStatefulSet("test-statefulset-"+suffix, ns.GetName())

		// Create fake client with the mapper and test resources
		Expect(c.Create(ctx, ns)).To(Succeed())
		Expect(c.Create(ctx, deployment)).To(Succeed())
		Expect(c.Create(ctx, statefulSet)).To(Succeed())

		// Initialize the CleanupService
		cleanupService = NewCleanupService(ctx, c, testEnv.Cfg)
	})

	Describe("CleanupItems", func() {
		It("should scale resources to zero when action is ScaleToZero", func() {
			items := []cleanupv1alpha1.PreClusterDestroyCleanupItem{
				{
					Kind:      "Deployment",
					Namespace: ns.GetName(),
					Name:      deployment.GetName(),
					Action:    cleanupv1alpha1.ActionScaleToZero,
				},
			}

			count, err := cleanupService.CleanupItems(ctx, false, items)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Verify the deployment was scaled to zero
			d := &appsv1.Deployment{}
			err = c.Get(ctx, client.ObjectKey{Namespace: ns.GetName(), Name: deployment.GetName()}, d)
			Expect(err).NotTo(HaveOccurred())
			Expect(*d.Spec.Replicas).To(Equal(int32(0)))
		})

		It("should delete resources when action is Delete", func() {
			items := []cleanupv1alpha1.PreClusterDestroyCleanupItem{
				{
					Kind:      "Deployment",
					Namespace: ns.GetName(),
					Name:      deployment.GetName(),
					Action:    cleanupv1alpha1.ActionDelete,
				},
			}

			count, err := cleanupService.CleanupItems(ctx, false, items)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Verify the deployment was deleted
			d := &appsv1.Deployment{}
			err = c.Get(ctx, client.ObjectKey{Namespace: ns.GetName(), Name: deployment.GetName()}, d)
			Expect(err).To(HaveOccurred())
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		It("should handle multiple items with different actions", func() {
			items := []cleanupv1alpha1.PreClusterDestroyCleanupItem{
				{
					Kind:      "Deployment",
					Namespace: ns.GetName(),
					Name:      deployment.GetName(),
					Action:    cleanupv1alpha1.ActionScaleToZero,
				},
				{
					Kind:      "StatefulSet",
					Namespace: ns.GetName(),
					Name:      statefulSet.GetName(),
					Action:    cleanupv1alpha1.ActionDelete,
				},
			}

			count, err := cleanupService.CleanupItems(ctx, false, items)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(2))

			// Verify the deployment was scaled to zero
			d := &appsv1.Deployment{}
			err = c.Get(ctx, client.ObjectKey{Namespace: ns.GetName(), Name: deployment.GetName()}, d)
			Expect(err).NotTo(HaveOccurred())
			Expect(*d.Spec.Replicas).To(Equal(int32(0)))

			// Verify the statefulset was deleted
			s := &appsv1.StatefulSet{}
			err = c.Get(ctx, client.ObjectKey{Namespace: ns.GetName(), Name: statefulSet.GetName()}, s)
			Expect(err).To(HaveOccurred())
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		It("should not make changes in dry run mode", func() {
			items := []cleanupv1alpha1.PreClusterDestroyCleanupItem{
				{
					Kind:      "Deployment",
					Namespace: ns.GetName(),
					Name:      deployment.GetName(),
					Action:    cleanupv1alpha1.ActionScaleToZero,
				},
				{
					Kind:      "StatefulSet",
					Namespace: ns.GetName(),
					Name:      statefulSet.GetName(),
					Action:    cleanupv1alpha1.ActionDelete,
				},
			}

			count, err := cleanupService.CleanupItems(ctx, true, items)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(2))

			// Verify the deployment was not changed
			d := &appsv1.Deployment{}
			err = c.Get(ctx, client.ObjectKey{Namespace: ns.GetName(), Name: deployment.GetName()}, d)
			Expect(err).NotTo(HaveOccurred())
			Expect(*d.Spec.Replicas).To(Equal(int32(3)))

			// Verify the statefulset was not deleted
			s := &appsv1.StatefulSet{}
			err = c.Get(ctx, client.ObjectKey{Namespace: ns.GetName(), Name: statefulSet.GetName()}, s)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle missing kind gracefully", func() {
			items := []cleanupv1alpha1.PreClusterDestroyCleanupItem{
				{
					Namespace: ns.GetName(),
					Name:      deployment.GetName(),
					Action:    cleanupv1alpha1.ActionScaleToZero,
				},
			}

			_, err := cleanupService.CleanupItems(ctx, false, items)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("kind must be specified for item"))
		})

		It("should handle unknown action gracefully", func() {
			items := []cleanupv1alpha1.PreClusterDestroyCleanupItem{
				{
					Kind:      "Deployment",
					Namespace: ns.GetName(),
					Name:      deployment.GetName(),
					Action:    cleanupv1alpha1.ActionUnknown,
				},
			}

			_, err := cleanupService.CleanupItems(ctx, false, items)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("action must be specified for item"))
		})
	})
})
