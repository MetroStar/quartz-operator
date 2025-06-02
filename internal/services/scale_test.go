package services

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cleanupv1alpha1 "github.com/MetroStar/quartz-operator/api/v1alpha1"
)

var _ = Describe("ScaleService", func() {
	var (
		ctx           context.Context
		c             client.Client
		scaleService  *ScaleService
		lookupService *LookupService
		ns            *corev1.Namespace
		deployment    *appsv1.Deployment
		statefulSet   *appsv1.StatefulSet
	)

	BeforeEach(func() {
		ctx = context.Background()
		c = testEnv.K8sClient

		// Create test objects
		t := testEnv.WithRandomSuffix()
		ns = t.Namespace("scaleservice")
		deployment = t.Deployment("test-deployment", ns.GetName())
		statefulSet = t.StatefulSet("test-statefulset", ns.GetName())

		Expect(c.Create(ctx, ns)).To(Succeed())
		Expect(c.Create(ctx, deployment)).To(Succeed())
		Expect(c.Create(ctx, statefulSet)).To(Succeed())

		// Initialize services
		lookupService = NewLookupService(c, t.Cfg)
		scaleService = NewScaleService(ctx, c, lookupService)
	})

	Describe("ScaleDeployment", func() {
		It("should scale a deployment when not in dry run mode", func() {
			replicas := testEnv.Int32Ptr(0)
			count, err := scaleService.ScaleDeployment(ctx, false, ns.GetName(), deployment.GetName(), replicas)

			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Verify the deployment was scaled
			d := &appsv1.Deployment{}
			err = c.Get(ctx, types.NamespacedName{Namespace: ns.GetName(), Name: deployment.GetName()}, d)
			Expect(err).NotTo(HaveOccurred())
			Expect(*d.Spec.Replicas).To(Equal(int32(0)))
		})

		It("should not actually scale a deployment in dry run mode", func() {
			replicas := testEnv.Int32Ptr(0)
			count, err := scaleService.ScaleDeployment(ctx, true, ns.GetName(), deployment.GetName(), replicas)

			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Verify the deployment was not actually scaled
			d := &appsv1.Deployment{}
			err = c.Get(ctx, types.NamespacedName{Namespace: ns.GetName(), Name: deployment.GetName()}, d)
			Expect(err).NotTo(HaveOccurred())
			Expect(*d.Spec.Replicas).To(Equal(int32(3))) // Should remain at original value
		})

		It("should return error when deployment doesn't exist", func() {
			replicas := testEnv.Int32Ptr(0)
			count, err := scaleService.ScaleDeployment(ctx, false, ns.GetName(), "nonexistent-deployment", replicas)

			Expect(err).To(HaveOccurred())
			Expect(count).To(Equal(0))
		})
	})

	Describe("ScaleStatefulSet", func() {
		It("should scale a statefulset when not in dry run mode", func() {
			replicas := testEnv.Int32Ptr(0)
			count, err := scaleService.ScaleStatefulSet(ctx, false, ns.GetName(), statefulSet.GetName(), replicas)

			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Verify the statefulset was scaled
			s := &appsv1.StatefulSet{}
			err = c.Get(ctx, types.NamespacedName{Namespace: ns.GetName(), Name: statefulSet.GetName()}, s)
			Expect(err).NotTo(HaveOccurred())
			Expect(*s.Spec.Replicas).To(Equal(int32(0)))
		})

		It("should not actually scale a statefulset in dry run mode", func() {
			replicas := testEnv.Int32Ptr(0)
			count, err := scaleService.ScaleStatefulSet(ctx, true, ns.GetName(), statefulSet.GetName(), replicas)

			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Verify the statefulset was not actually scaled
			s := &appsv1.StatefulSet{}
			err = c.Get(ctx, types.NamespacedName{Namespace: ns.GetName(), Name: statefulSet.GetName()}, s)
			Expect(err).NotTo(HaveOccurred())
			Expect(*s.Spec.Replicas).To(Equal(int32(3))) // Should remain at original value
		})

		It("should return error when statefulset doesn't exist", func() {
			replicas := testEnv.Int32Ptr(0)
			count, err := scaleService.ScaleStatefulSet(ctx, false, ns.GetName(), "nonexistent-statefulset", replicas)

			Expect(err).To(HaveOccurred())
			Expect(count).To(Equal(0))
		})
	})

	Describe("ScaleItem", func() {
		It("should scale a deployment by name", func() {
			replicas := testEnv.Int32Ptr(0)
			item := cleanupv1alpha1.PreClusterDestroyCleanupItem{
				Kind:      DeploymentKind,
				Namespace: ns.GetName(),
				Name:      deployment.GetName(),
				Action:    cleanupv1alpha1.ActionScaleToZero,
			}

			count, err := scaleService.ScaleItem(ctx, false, schema.GroupVersionKind{Kind: DeploymentKind}, item, replicas)

			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Verify the deployment was scaled
			d := &appsv1.Deployment{}
			err = c.Get(ctx, types.NamespacedName{Namespace: ns.GetName(), Name: deployment.GetName()}, d)
			Expect(err).NotTo(HaveOccurred())
			Expect(*d.Spec.Replicas).To(Equal(int32(0)))
		})

		It("should scale a statefulset by name", func() {
			replicas := testEnv.Int32Ptr(0)
			item := cleanupv1alpha1.PreClusterDestroyCleanupItem{
				Kind:      StatefulSetKind,
				Namespace: ns.GetName(),
				Name:      statefulSet.GetName(),
				Action:    cleanupv1alpha1.ActionScaleToZero,
			}

			count, err := scaleService.ScaleItem(ctx, false, schema.GroupVersionKind{Kind: StatefulSetKind}, item, replicas)

			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Verify the statefulset was scaled
			s := &appsv1.StatefulSet{}
			err = c.Get(ctx, types.NamespacedName{Namespace: ns.GetName(), Name: statefulSet.GetName()}, s)
			Expect(err).NotTo(HaveOccurred())
			Expect(*s.Spec.Replicas).To(Equal(int32(0)))
		})

		It("should not support scaling for unsupported kinds", func() {
			replicas := testEnv.Int32Ptr(0)
			item := cleanupv1alpha1.PreClusterDestroyCleanupItem{
				Kind:      "Service",
				Namespace: ns.GetName(),
				Name:      "test-service",
				Action:    cleanupv1alpha1.ActionScaleToZero,
			}

			_, err := scaleService.ScaleItem(ctx, false, schema.GroupVersionKind{Kind: "Service"}, item, replicas)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("scaling is not supported for kind Service"))
		})
	})
})
