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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cleanupv1alpha1 "github.com/MetroStar/quartz-operator/api/v1alpha1"
)

var _ = Describe("PreClusterDestroyCleanup Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	var (
		resourceName       string
		ns                 *corev1.Namespace
		typeNamespacedName types.NamespacedName
		deployment         *appsv1.Deployment
		statefulSet        *appsv1.StatefulSet
	)

	ctx := context.Background()

	BeforeEach(func() {
		By("creating the custom resource for the Kind PreClusterDestroyCleanup")

		t := sharedTestEnv.WithRandomSuffix()
		resourceName = t.FormatName("test-resource")
		ns = t.Namespace("test-namespace")

		Expect(k8sClient.Create(ctx, ns)).To(Succeed())

		typeNamespacedName = types.NamespacedName{
			Name:      resourceName,
			Namespace: ns.GetName(),
		}

		// Create a test deployment
		deployment = t.Deployment("test-deployment", ns.GetName())
		Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

		// Create a test statefulset
		statefulSet = t.StatefulSet("test-stateful", ns.GetName())
		Expect(k8sClient.Create(ctx, statefulSet)).To(Succeed())
	})

	AfterEach(func() {
		By("Cleanup the test namespace")
		Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
	})

	It("should successfully reconcile the resource", func() {
		By("Reconciling the created resource")
		controllerReconciler := &PreClusterDestroyCleanupReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: typeNamespacedName,
		})
		Expect(err).NotTo(HaveOccurred())
		// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
		// Example: If you expect a certain status condition after reconciliation, verify it here.
	})
	Context("When reconciling a resource with no cleanup items", func() {
		var preclusterdestroycleanup *cleanupv1alpha1.PreClusterDestroyCleanup

		BeforeEach(func() {
			By("creating the custom resource for the Kind PreClusterDestroyCleanup with no items")
			preclusterdestroycleanup = &cleanupv1alpha1.PreClusterDestroyCleanup{}
			err := k8sClient.Get(ctx, typeNamespacedName, preclusterdestroycleanup)
			if err != nil && errors.IsNotFound(err) {
				resource := &cleanupv1alpha1.PreClusterDestroyCleanup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: ns.GetName(),
					},
					Spec: cleanupv1alpha1.PreClusterDestroyCleanupSpec{
						DryRun:    false,
						Resources: []cleanupv1alpha1.PreClusterDestroyCleanupItem{},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
				preclusterdestroycleanup = resource
			}
		})

		AfterEach(func() {
			resource := &cleanupv1alpha1.PreClusterDestroyCleanup{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance PreClusterDestroyCleanup")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should set status to Complete with NoResources reason", func() {
			By("Reconciling the created resource")
			controllerReconciler := &PreClusterDestroyCleanupReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Config: cfg,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Fetch the updated resource
			updatedResource := &cleanupv1alpha1.PreClusterDestroyCleanup{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, typeNamespacedName, updatedResource); err != nil {
					return false
				}
				condition := meta.FindStatusCondition(updatedResource.Status.Conditions, ConditionComplete)
				return condition != nil && condition.Reason == ReasonNoResources
			}, timeout, interval).Should(BeTrue())

			// Verify the condition details
			condition := meta.FindStatusCondition(updatedResource.Status.Conditions, ConditionComplete)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal(ReasonNoResources))
			Expect(condition.Message).To(Equal("No resources specified for processing"))
		})
	})

	Context("When reconciling a resource with ScaleToZero action", func() {
		BeforeEach(func() {
			By("creating the custom resource for the Kind PreClusterDestroyCleanup with ScaleToZero action")
			resource := &cleanupv1alpha1.PreClusterDestroyCleanup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: ns.GetName(),
				},
				Spec: cleanupv1alpha1.PreClusterDestroyCleanupSpec{
					DryRun: false,
					Resources: []cleanupv1alpha1.PreClusterDestroyCleanupItem{
						{
							Kind:      "Deployment",
							Namespace: ns.GetName(),
							Name:      deployment.GetName(),
							Action:    cleanupv1alpha1.ActionScaleToZero,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		It("should scale the deployment to zero and update status", func() {
			By("Reconciling the created resource")
			controllerReconciler := &PreClusterDestroyCleanupReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Config: cfg,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify the deployment was scaled to zero
			d := &appsv1.Deployment{}
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: deployment.GetName(), Namespace: ns.GetName()}, d)
				if err != nil {
					return -1
				}
				return *d.Spec.Replicas
			}, timeout, interval).Should(Equal(int32(0)))

			// Verify the status was updated correctly
			updatedResource := &cleanupv1alpha1.PreClusterDestroyCleanup{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, typeNamespacedName, updatedResource); err != nil {
					return false
				}
				condition := meta.FindStatusCondition(updatedResource.Status.Conditions, ConditionComplete)
				return condition != nil && condition.Reason == ReasonCompletedSuccessfully
			}, timeout, interval).Should(BeTrue())

			// Verify the condition details
			condition := meta.FindStatusCondition(updatedResource.Status.Conditions, ConditionComplete)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal(ReasonCompletedSuccessfully))
			Expect(condition.Message).To(ContainSubstring("Processed"))
		})
	})

	Context("When reconciling a resource with Delete action", func() {
		var preclusterdestroycleanup *cleanupv1alpha1.PreClusterDestroyCleanup

		BeforeEach(func() {
			By("creating the custom resource for the Kind PreClusterDestroyCleanup with Delete action")
			preclusterdestroycleanup = &cleanupv1alpha1.PreClusterDestroyCleanup{}
			err := k8sClient.Get(ctx, typeNamespacedName, preclusterdestroycleanup)
			if err != nil && errors.IsNotFound(err) {
				resource := &cleanupv1alpha1.PreClusterDestroyCleanup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: ns.GetName(),
					},
					Spec: cleanupv1alpha1.PreClusterDestroyCleanupSpec{
						DryRun: false,
						Resources: []cleanupv1alpha1.PreClusterDestroyCleanupItem{
							{
								Kind:      "StatefulSet",
								Namespace: ns.GetName(),
								Name:      statefulSet.GetName(),
								Action:    cleanupv1alpha1.ActionDelete,
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
				preclusterdestroycleanup = resource
			}
		})

		It("should delete the statefulset and update status", func() {
			By("Reconciling the created resource")
			controllerReconciler := &PreClusterDestroyCleanupReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Config: cfg,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify the statefulset was deleted
			s := &appsv1.StatefulSet{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: statefulSet.GetName(), Namespace: ns.GetName()}, s)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			// Verify the status was updated correctly
			updatedResource := &cleanupv1alpha1.PreClusterDestroyCleanup{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, typeNamespacedName, updatedResource); err != nil {
					return false
				}
				condition := meta.FindStatusCondition(updatedResource.Status.Conditions, ConditionComplete)
				return condition != nil && condition.Reason == ReasonCompletedSuccessfully
			}, timeout, interval).Should(BeTrue())

			// Verify the condition details
			condition := meta.FindStatusCondition(updatedResource.Status.Conditions, ConditionComplete)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal(ReasonCompletedSuccessfully))
			Expect(condition.Message).To(ContainSubstring("Processed"))
		})
	})

	Context("When reconciling a resource with DryRun mode enabled", func() {
		var preclusterdestroycleanup *cleanupv1alpha1.PreClusterDestroyCleanup

		BeforeEach(func() {
			By("creating the custom resource for the Kind PreClusterDestroyCleanup with DryRun enabled")
			preclusterdestroycleanup = &cleanupv1alpha1.PreClusterDestroyCleanup{}
			err := k8sClient.Get(ctx, typeNamespacedName, preclusterdestroycleanup)
			if err != nil && errors.IsNotFound(err) {
				resource := &cleanupv1alpha1.PreClusterDestroyCleanup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: ns.GetName(),
					},
					Spec: cleanupv1alpha1.PreClusterDestroyCleanupSpec{
						DryRun: true,
						Resources: []cleanupv1alpha1.PreClusterDestroyCleanupItem{
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
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
				preclusterdestroycleanup = resource
			}
		})

		It("should update status but not modify resources in DryRun mode", func() {
			By("Reconciling the created resource")
			controllerReconciler := &PreClusterDestroyCleanupReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Config: cfg,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify that deployment replicas were NOT changed (should still be 3)
			d := &appsv1.Deployment{}
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: deployment.GetName(), Namespace: ns.GetName()}, d)
				if err != nil {
					return -1
				}
				return *d.Spec.Replicas
			}, timeout, interval).Should(Equal(int32(3)))

			// Verify that statefulset was NOT deleted
			s := &appsv1.StatefulSet{}
			Consistently(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: statefulSet.GetName(), Namespace: ns.GetName()}, s)
			}, timeout, interval).Should(Succeed())

			// Verify the status was updated correctly
			updatedResource := &cleanupv1alpha1.PreClusterDestroyCleanup{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, typeNamespacedName, updatedResource); err != nil {
					return false
				}
				condition := meta.FindStatusCondition(updatedResource.Status.Conditions, ConditionComplete)
				return condition != nil && condition.Reason == ReasonCompletedSuccessfully
			}, timeout, interval).Should(BeTrue())

			// Verify the condition details
			condition := meta.FindStatusCondition(updatedResource.Status.Conditions, ConditionComplete)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal(ReasonCompletedSuccessfully))
			Expect(condition.Message).To(ContainSubstring("Processed"))
		})
	})

	Context("When reconciling a resource with non-existent resources", func() {
		BeforeEach(func() {
			By("creating the custom resource for the Kind PreClusterDestroyCleanup with non-existent resources")
			resource := &cleanupv1alpha1.PreClusterDestroyCleanup{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err != nil && errors.IsNotFound(err) {
				resource = &cleanupv1alpha1.PreClusterDestroyCleanup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: ns.GetName(),
					},
					Spec: cleanupv1alpha1.PreClusterDestroyCleanupSpec{
						DryRun: false,
						Resources: []cleanupv1alpha1.PreClusterDestroyCleanupItem{
							{
								Kind:      "Deployment",
								Namespace: ns.GetName(),
								Name:      "non-existent-deployment",
								Action:    cleanupv1alpha1.ActionScaleToZero,
							},
							{
								Kind:      "StatefulSet",
								Namespace: ns.GetName(),
								Name:      "non-existent-statefulset",
								Action:    cleanupv1alpha1.ActionDelete,
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		It("should handle errors gracefully and update status with error information", func() {
			By("Reconciling the created resource")
			controllerReconciler := &PreClusterDestroyCleanupReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Config: cfg,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			// The controller handles errors internally by updating the status
			// So the reconcile itself shouldn't error out
			Expect(err).To(HaveOccurred())

			// Verify the status indicates errors
			updatedResource := &cleanupv1alpha1.PreClusterDestroyCleanup{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, typeNamespacedName, updatedResource); err != nil {
					return false
				}
				condition := meta.FindStatusCondition(updatedResource.Status.Conditions, ConditionComplete)
				return condition != nil && condition.Reason == ReasonCompletedWithErrors
			}, timeout, interval).Should(BeTrue())

			// Verify the condition details
			condition := meta.FindStatusCondition(updatedResource.Status.Conditions, ConditionComplete)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal(ReasonCompletedWithErrors))
			Expect(condition.Message).To(ContainSubstring("error"))
		})
	})

	Context("When reconciling a resource with invalid actions", func() {
		BeforeEach(func() {
			By("creating the custom resource for the Kind PreClusterDestroyCleanup with invalid actions")
			resource := &cleanupv1alpha1.PreClusterDestroyCleanup{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err != nil && errors.IsNotFound(err) {
				resource = &cleanupv1alpha1.PreClusterDestroyCleanup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: ns.GetName(),
					},
					Spec: cleanupv1alpha1.PreClusterDestroyCleanupSpec{
						DryRun: false,
						Resources: []cleanupv1alpha1.PreClusterDestroyCleanupItem{
							{
								Kind:      "Deployment",
								Namespace: ns.GetName(),
								Name:      deployment.GetName(),
								Action:    cleanupv1alpha1.ActionUnknown, // Empty action
							},
							{
								Kind:      "Service", // Kind that doesn't support scaling
								Namespace: ns.GetName(),
								Name:      "test-service",
								Action:    cleanupv1alpha1.ActionScaleToZero,
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		It("should handle invalid actions and update status with error information", func() {
			By("Reconciling the created resource")
			controllerReconciler := &PreClusterDestroyCleanupReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Config: cfg,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			// The controller handles errors internally by updating the status
			// So the reconcile itself shouldn't error out
			Expect(err).To(HaveOccurred())

			// Verify the status indicates errors
			updatedResource := &cleanupv1alpha1.PreClusterDestroyCleanup{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, typeNamespacedName, updatedResource); err != nil {
					return false
				}
				condition := meta.FindStatusCondition(updatedResource.Status.Conditions, ConditionComplete)
				return condition != nil && condition.Reason == ReasonCompletedWithErrors
			}, timeout, interval).Should(BeTrue())

			// Verify the condition details
			condition := meta.FindStatusCondition(updatedResource.Status.Conditions, ConditionComplete)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal(ReasonCompletedWithErrors))
			Expect(condition.Message).To(ContainSubstring("error"))
		})
	})
})
