package services

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cleanupv1alpha1 "github.com/MetroStar/quartz-operator/api/v1alpha1"
)

var _ = Describe("UpdateService", func() {
	var (
		ctx context.Context
		c   client.Client
		svc *UpdateService
		ns  *corev1.Namespace
		obj *cleanupv1alpha1.PreClusterDestroyCleanup
	)

	BeforeEach(func() {
		ctx = testEnv.Ctx
		c = testEnv.K8sClient

		t := testEnv.WithRandomSuffix()
		ns = t.Namespace("deleteservice")
		obj = &cleanupv1alpha1.PreClusterDestroyCleanup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      t.FormatName("test-cleanup"),
				Namespace: ns.GetName(),
			},
			Spec: cleanupv1alpha1.PreClusterDestroyCleanupSpec{
				DryRun: false,
				Resources: []cleanupv1alpha1.PreClusterDestroyCleanupItem{
					{
						Kind:      "Deployment",
						Namespace: "test-ns",
						Name:      t.FormatName("test-deploy"),
						Action:    cleanupv1alpha1.ActionDelete,
					},
				},
			},
			Status: cleanupv1alpha1.PreClusterDestroyCleanupStatus{
				Conditions: []metav1.Condition{},
			},
		}

		Expect(c.Create(ctx, ns)).To(Succeed())
		Expect(c.Create(ctx, obj)).To(Succeed())
		svc = NewUpdateService(c)
	})

	Context("UpdateCondition", func() {
		It("should successfully update the status condition", func() {
			// Call the service method
			err := svc.UpdateCondition(ctx, obj, "TestCondition", "TestReason", "This is a test message")
			Expect(err).NotTo(HaveOccurred())

			// Verify that the condition was set
			condition := meta.FindStatusCondition(obj.Status.Conditions, "TestCondition")
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("TestReason"))
			Expect(condition.Message).To(Equal("This is a test message"))

			// Verify that the object was updated in the fake client
			updatedObj := &cleanupv1alpha1.PreClusterDestroyCleanup{}
			err = c.Get(ctx, client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}, updatedObj)
			Expect(err).NotTo(HaveOccurred())

			updatedCondition := meta.FindStatusCondition(updatedObj.Status.Conditions, "TestCondition")
			Expect(updatedCondition).NotTo(BeNil())
			Expect(updatedCondition.Status).To(Equal(metav1.ConditionTrue))
			Expect(updatedCondition.Reason).To(Equal("TestReason"))
			Expect(updatedCondition.Message).To(Equal("This is a test message"))
		})

		It("should handle client update errors", func() {
			// Create a fake client that returns an error on Status().Update()
			errorClient := &fakeErrorClient{Client: c}
			errorSvc := NewUpdateService(errorClient)

			// Call the service method
			err := errorSvc.UpdateCondition(ctx, obj, "TestCondition", "TestReason", "This is a test message")

			// Should return an error
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to update PreClusterDestroyCleanup status"))
		})

		It("should add multiple conditions", func() {
			// Set the first condition
			err := svc.UpdateCondition(ctx, obj, "FirstCondition", "FirstReason", "First message")
			Expect(err).NotTo(HaveOccurred())

			// Set a second condition
			err = svc.UpdateCondition(ctx, obj, "SecondCondition", "SecondReason", "Second message")
			Expect(err).NotTo(HaveOccurred())

			// Verify both conditions are set
			condition1 := meta.FindStatusCondition(obj.Status.Conditions, "FirstCondition")
			Expect(condition1).NotTo(BeNil())
			Expect(condition1.Reason).To(Equal("FirstReason"))

			condition2 := meta.FindStatusCondition(obj.Status.Conditions, "SecondCondition")
			Expect(condition2).NotTo(BeNil())
			Expect(condition2.Reason).To(Equal("SecondReason"))
		})

		It("should update an existing condition", func() {
			// Set initial condition
			err := svc.UpdateCondition(ctx, obj, "TestCondition", "InitialReason", "Initial message")
			Expect(err).NotTo(HaveOccurred())

			// Update the same condition
			err = svc.UpdateCondition(ctx, obj, "TestCondition", "UpdatedReason", "Updated message")
			Expect(err).NotTo(HaveOccurred())

			// Verify the condition was updated
			condition := meta.FindStatusCondition(obj.Status.Conditions, "TestCondition")
			Expect(condition).NotTo(BeNil())
			Expect(condition.Reason).To(Equal("UpdatedReason"))
			Expect(condition.Message).To(Equal("Updated message"))
		})
	})
})

// fakeErrorClient is a mock client that returns an error on Status().Update()
type fakeErrorClient struct {
	client.Client
}

// Status returns a StatusWriter that always returns an error on Update
func (f *fakeErrorClient) Status() client.StatusWriter {
	return &fakeErrorStatusWriter{}
}

type fakeErrorStatusWriter struct {
	client.StatusWriter
}

// Update always returns an error
func (f *fakeErrorStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return errors.New("forced error from fake client")
}

// Patch is not used in our tests but included to satisfy the interface
func (f *fakeErrorStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	return errors.New("forced error from fake client")
}
