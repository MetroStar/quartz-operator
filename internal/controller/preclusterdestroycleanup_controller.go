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
	"errors"
	"fmt"
	"slices"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	apiextclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cleanupv1alpha1 "github.com/MetroStar/quartz-operator/api/v1alpha1"
)

// PreClusterDestroyCleanupReconciler reconciles a PreClusterDestroyCleanup object
type PreClusterDestroyCleanupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

// +kubebuilder:rbac:groups=cleanup.quartz.metrostar.com,resources=preclusterdestroycleanups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cleanup.quartz.metrostar.com,resources=preclusterdestroycleanups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cleanup.quartz.metrostar.com,resources=preclusterdestroycleanups/finalizers,verbs=update
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=list;get;watch
// +kubebuilder:rbac:groups=*,resources=*,verbs=delete;list;get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *PreClusterDestroyCleanupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling PreClusterDestroyCleanup", "name", req.Name, "namespace", req.Namespace)

	obj := &cleanupv1alpha1.PreClusterDestroyCleanup{}
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		logger.Error(err, "unable to fetch PreClusterDestroyCleanup")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if len(obj.Status.Conditions) == 0 {
		// Initialize conditions if not set
		if err := r.updateCondition(ctx, obj, "Initialized", "Reconciling", "Reconciliation started"); err != nil {
			logger.Error(err, "failed to initialize PreClusterDestroyCleanup status conditions")
			return ctrl.Result{}, err
		}
		logger.Info("Initialized status conditions for PreClusterDestroyCleanup", "name", req.Name, "namespace", req.Namespace)

		if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
			logger.Error(err, "failed to re-fetch PreClusterDestroyCleanup after initializing status conditions")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	}

	items := obj.Spec.Resources
	if len(items) == 0 {
		logger.Info("No resources specified, skipping cleanup")
		if err := r.updateCondition(ctx, obj, "Complete", "NoResources", "No resources specified for cleanup"); err != nil {
			logger.Error(err, "failed to update PreClusterDestroyCleanup status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	count := 0
	errs := []error{}
	for _, item := range items {
		if item.Kind == "" {
			errs = append(errs, fmt.Errorf("kind must be specified for item: %v", item))
			continue
		}

		gvk, err := r.lookupGroupKind(item.Kind)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to lookup group and kind for %s: %w", item.Kind, err))
			continue
		}

		c := 0

		if item.Action == "" {
			errs = append(errs, fmt.Errorf("action must be specified for item: %v", item))
			continue
		}

		if item.Action == "scaleToZero" {
			logger.Info("Scaling to zero", "kind", gvk.Kind, "namespace", item.Namespace, "name", item.Name)
			c, err := r.scaleToZero(ctx, obj.Spec.DryRun, gvk, item.Namespace, item.Name)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to scale %s/%s to zero: %w", item.Namespace, item.Name, err))
			}
			count += c
			continue
		}

		// special case to handle deletion of all resources of crds with a specific category, ex. "kubectl get managed"
		if gvk.Kind == "CustomResourceDefinition" && item.Category != "" {
			logger.Info("Looking up CRDs by category", "category", item.Category)
			gvks, err := r.lookupCrdsByCategory(ctx, item.Category)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to lookup CRDs by category %s: %w", item.Category, err))
				continue
			}
			if len(gvks) == 0 {
				errs = append(errs, fmt.Errorf("no CRDs found for category %s", item.Category))
				continue
			}
			for _, gvk := range gvks {
				c, err = r.cleanupResources(ctx, obj.Spec.DryRun, gvk, item.Namespace)
				if err != nil {
					errs = append(errs, fmt.Errorf("failed to cleanup resources of kind %s in namespace %s: %w", gvk.Kind, item.Namespace, err))
					continue
				}
				count += c
			}
			continue
		}

		// if no name was specified, delete all resources of the specified kind, optionally scoped to a namespace
		if item.Name == "" {
			if c, err = r.cleanupResources(ctx, obj.Spec.DryRun, gvk, item.Namespace); err != nil {
				errs = append(errs, fmt.Errorf("failed to cleanup resources of kind %s in namespace %s: %w", item.Kind, item.Namespace, err))
			}
			count += c
			continue
		}

		// if a name was specified, delete the specific resource
		if c, err = r.cleanupNamedResource(ctx, obj.Spec.DryRun, gvk, item.Namespace, item.Name); err != nil {
			errs = append(errs, fmt.Errorf("failed to cleanup named resource %s/%s: %w", item.Namespace, item.Name, err))
		}
		count += c
	}

	if len(errs) > 0 {
		logger.Error(errors.Join(errs...), "Errors occurred during cleanup")
		if err := r.updateCondition(ctx, obj, "Complete", "CompletedWithErrors", fmt.Sprintf("Errors occurred during cleanup: %d errors", len(errs))); err != nil {
			logger.Error(err, "failed to update PreClusterDestroyCleanup status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, errors.Join(errs...)
	}

	if err := r.updateCondition(ctx, obj, "Complete", "CompletedSuccessfully", fmt.Sprintf("Cleaned up %d resources", count)); err != nil {
		logger.Error(err, "failed to update PreClusterDestroyCleanup status")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciliation complete for PreClusterDestroyCleanup", "name", req.Name, "namespace", req.Namespace)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PreClusterDestroyCleanupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cleanupv1alpha1.PreClusterDestroyCleanup{}).
		Named("preclusterdestroycleanup").
		Complete(r)
}

// lookupGroupKind attempts to find the GroupVersionKind for a given kind.
// It first tries to find it directly by kind, and if that fails, it tries to find it by group and kind.
func (r *PreClusterDestroyCleanupReconciler) lookupGroupKind(kind string) (schema.GroupVersionKind, error) {
	mapper := r.Client.RESTMapper()

	// try to find the resource directly by kind
	k, err := mapper.KindFor(schema.GroupVersionResource{Resource: kind})
	if err == nil {
		return k, nil
	}

	// if not found, try to find it by group and kind
	gk := schema.ParseGroupKind(kind)
	m, err := mapper.RESTMapping(gk)
	if err == nil {
		return m.GroupVersionKind, nil
	}

	return schema.GroupVersionKind{}, fmt.Errorf("failed to find mapping for kind %s: %w", kind, err)
}

// lookupCrdsByCategory looks up CustomResourceDefinitions (CRDs) by their category.
// It returns a slice of GroupVersionKind for CRDs that match the specified category.
func (r *PreClusterDestroyCleanupReconciler) lookupCrdsByCategory(ctx context.Context, category string) ([]schema.GroupVersionKind, error) {
	crdClient, err := apiextclient.NewForConfig(r.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create CRD client: %w", err)
	}

	crds, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list CRDs: %w", err)
	}

	gvks := []schema.GroupVersionKind{}
	logger := log.FromContext(ctx)
	for _, crd := range crds.Items {
		if slices.ContainsFunc(crd.Spec.Names.Categories, func(c string) bool {
			return strings.EqualFold(c, category)
		}) {
			logger.Info("Found CRD with category", "name", crd.Name, "categories", crd.Spec.Names.Categories)
			gvks = append(gvks, schema.GroupVersionKind{
				Group:   crd.Spec.Group,
				Version: crd.Spec.Versions[0].Name, // Use the first version for simplicity
				Kind:    crd.Spec.Names.Kind,
			})
		}
	}

	return gvks, nil
}

// cleanupResources deletes all resources of a specific kind in a given namespace.
// It returns the count of deleted resources and any errors encountered during deletion.
// If dryRun is true, it only logs the resources that would be deleted without actually deleting them.
func (r *PreClusterDestroyCleanupReconciler) cleanupResources(ctx context.Context, dryRun bool, gvk schema.GroupVersionKind, ns string) (int, error) {
	list, err := r.listResources(ctx, gvk, ns)
	if err != nil {
		return 0, fmt.Errorf("failed to list resources of kind %s in namespace %s: %w", gvk.Kind, ns, err)
	}

	logger := log.FromContext(ctx)
	if len(list.Items) == 0 {
		logger.Info("No resources found to delete", "kind", gvk.Kind, "namespace", ns)
		return 0, nil // Nothing to delete
	}

	if dryRun {
		logger.Info("Dry run mode, skipping deletion", "kind", gvk.Kind, "namespace", ns, "count", len(list.Items))
		for _, item := range list.Items {
			logger.Info("Would delete item", "kind", gvk.Kind, "namespace", item.GetNamespace(), "name", item.GetName())
		}
		return len(list.Items), nil
	}

	count := 0
	errs := []error{}
	for _, item := range list.Items {
		if err := r.Client.Delete(ctx, &item); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete %s/%s: %w", item.GetNamespace(), item.GetName(), err))
			continue
		}
		count++
	}

	return count, errors.Join(errs...)
}

// cleanupNamedResource deletes a specific resource by its kind, namespace, and name.
// It returns the count of deleted resources (1 if successful, 0 if not found) and any errors encountered during deletion.
// If dryRun is true, it only logs the resource that would be deleted without actually deleting it.
func (r *PreClusterDestroyCleanupReconciler) cleanupNamedResource(ctx context.Context, dryRun bool, gvk schema.GroupVersionKind, ns string, name string) (int, error) {
	item := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       gvk.Kind,
			APIVersion: gvk.GroupVersion().String(),
		},
	}

	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, item); err != nil {
		return 0, fmt.Errorf("failed to get %s/%s: %w", ns, name, err)
	}

	if dryRun {
		logger := log.FromContext(ctx)
		logger.Info("Dry run mode, skipping deletion", "kind", gvk.Kind, "namespace", ns, "name", name)
		logger.Info("Would delete item", "kind", gvk.Kind, "namespace", item.GetNamespace(), "name", item.GetName())
		return 1, nil
	}

	if err := r.Client.Delete(ctx, item); err != nil {
		return 0, fmt.Errorf("failed to delete %s/%s: %w", item.GetNamespace(), item.GetName(), err)
	}

	return 1, nil
}

// scaleToZero scales a resource down to zero replicas if it is a Deployment or StatefulSet.
// It returns the count of scaled resources (1 if successful, 0 if not applicable) and any errors encountered during scaling.
// If dryRun is true, it only logs the action without actually scaling the resource.
func (r *PreClusterDestroyCleanupReconciler) scaleToZero(ctx context.Context, dryRun bool, gvk schema.GroupVersionKind, ns string, name string) (int, error) {
	if gvk.Kind != "Deployment" && gvk.Kind != "StatefulSet" {
		return 0, fmt.Errorf("scaling to zero is not supported for kind %s", gvk.Kind)
	}

	logger := log.FromContext(ctx)
	if name != "" {
		c, err := r.scaleResourceToZero(ctx, dryRun, gvk, ns, name)
		if err != nil {
			return 0, fmt.Errorf("failed to scale %s/%s to zero: %w", ns, name, err)
		}
		return c, nil
	}

	// lookup all resources of the specified kind in the namespace
	list, err := r.listResources(ctx, gvk, ns)
	if err != nil {
		return 0, fmt.Errorf("failed to list resources of kind %s in namespace %s: %w", gvk.Kind, ns, err)
	}

	if len(list.Items) == 0 {
		logger.Info("No resources found to scale to zero", "kind", gvk.Kind, "namespace", ns)
		return 0, nil // Nothing to scale
	}

	count := 0
	errs := []error{}
	for _, item := range list.Items {
		c, err := r.scaleResourceToZero(ctx, dryRun, gvk, item.GetNamespace(), item.GetName())
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to scale %s/%s to zero: %w", item.GetNamespace(), item.GetName(), err))
		}
		count += c
	}

	return count, errors.Join(errs...)
}

// updateCondition updates the status condition of a PreClusterDestroyCleanup object.
// It sets the condition type, status, reason, and message, and updates the object status in the cluster.
// If the update fails, it returns an error.
func (r *PreClusterDestroyCleanupReconciler) updateCondition(ctx context.Context, obj *cleanupv1alpha1.PreClusterDestroyCleanup, t string, reason string, message string) error {
	meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type:    t,
		Status:  metav1.ConditionTrue,
		Reason:  reason,
		Message: message,
	})
	if err := r.Client.Status().Update(ctx, obj); err != nil {
		return fmt.Errorf("failed to update PreClusterDestroyCleanup status: %w", err)
	}

	return nil
}

func (r *PreClusterDestroyCleanupReconciler) listResources(ctx context.Context, gvk schema.GroupVersionKind, ns string) (*metav1.PartialObjectMetadataList, error) {
	list := &metav1.PartialObjectMetadataList{}

	v := gvk.Version
	if v == "" {
		v = "v1" // Default to v1 if no version is specified
	}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: v,
		Kind:    gvk.Kind + "List",
	})

	opts := []client.ListOption{
		client.InNamespace(ns),
	}

	if err := r.Client.List(ctx, list, opts...); err != nil {
		return nil, fmt.Errorf("failed to list %s in namespace %s: %w", gvk.Kind, ns, err)
	}

	return list, nil
}

func (r *PreClusterDestroyCleanupReconciler) scaleResourceToZero(ctx context.Context, dryRun bool, gvk schema.GroupVersionKind, ns string, name string) (int, error) {
	switch gvk.Kind {
	case "Deployment":
		return r.scaleDeploymentToZero(ctx, dryRun, gvk, ns, name)
	case "StatefulSet":
		return r.scaleStatefulSetToZero(ctx, dryRun, gvk, ns, name)
	default:
		return 0, fmt.Errorf("scaling to zero is not supported for kind %s", gvk.Kind)
	}
}

func (r *PreClusterDestroyCleanupReconciler) scaleDeploymentToZero(ctx context.Context, dryRun bool, gvk schema.GroupVersionKind, ns string, name string) (int, error) {
	if gvk.Kind != "Deployment" {
		return 0, fmt.Errorf("scaling to zero is not supported for kind %s", gvk.Kind)
	}

	logger := log.FromContext(ctx)
	if name == "" {
		logger.Info("No name specified for scaling to zero, skipping")
		return 0, nil // Nothing to scale
	}

	if dryRun {
		logger.Info("Dry run mode, skipping scaling to zero", "kind", gvk.Kind, "namespace", ns, "name", name)
		return 1, nil // Indicate that we would scale to zero
	}

	deployment := &appsv1.Deployment{}
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, deployment); err != nil {
		return 0, fmt.Errorf("failed to get %s/%s: %w", ns, name, err)
	}

	deployment.Spec.Replicas = nil // Set replicas to nil to scale down to zero
	if err := r.Client.Update(ctx, deployment); err != nil {
		return 0, fmt.Errorf("failed to scale %s/%s to zero: %w", ns, name, err)
	}

	logger.Info("Scaled deployment to zero", "kind", gvk.Kind, "namespace", ns, "name", name)
	return 1, nil // Indicate that we scaled to zero
}

// scaleStatefulSetToZero scales a StatefulSet down to zero replicas.
func (r *PreClusterDestroyCleanupReconciler) scaleStatefulSetToZero(ctx context.Context, dryRun bool, gvk schema.GroupVersionKind, ns string, name string) (int, error) {
	if gvk.Kind != "StatefulSet" {
		return 0, fmt.Errorf("scaling to zero is not supported for kind %s", gvk.Kind)
	}

	logger := log.FromContext(ctx)
	if name == "" {
		logger.Info("No name specified for scaling to zero, skipping")
		return 0, nil // Nothing to scale
	}

	if dryRun {
		logger.Info("Dry run mode, skipping scaling to zero", "kind", gvk.Kind, "namespace", ns, "name", name)
		return 1, nil // Indicate that we would scale to zero
	}

	statefulSet := &appsv1.StatefulSet{}
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, statefulSet); err != nil {
		return 0, fmt.Errorf("failed to get %s/%s: %w", ns, name, err)
	}

	statefulSet.Spec.Replicas = nil // Set replicas to nil to scale down to zero
	if err := r.Client.Update(ctx, statefulSet); err != nil {
		return 0, fmt.Errorf("failed to scale %s/%s to zero: %w", ns, name, err)
	}

	logger.Info("Scaled statefulset to zero", "kind", gvk.Kind, "namespace", ns, "name", name)
	return 1, nil // Indicate that we scaled to zero
}
