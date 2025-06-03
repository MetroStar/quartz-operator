package services

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cleanupv1alpha1 "github.com/MetroStar/quartz-operator/api/v1alpha1"
	"github.com/go-logr/logr"
)

// DeleteService provides methods to delete resources based on PreClusterDestroyCleanupItems.
type DeleteService struct {
	client client.Client
	lookup *LookupService
	logger logr.Logger
}

// NewDeleteService creates a new DeleteService instance.
func NewDeleteService(ctx context.Context, client client.Client, lookup *LookupService) *DeleteService {
	return &DeleteService{
		client: client,
		lookup: lookup,
		logger: log.FromContext(ctx),
	}
}

func (s *DeleteService) DeleteItem(ctx context.Context, dryRun bool, gvk schema.GroupVersionKind, item cleanupv1alpha1.PreClusterDestroyCleanupItem) (int, error) {
	// special case to handle deletion of all resources of crds with a specific category, ex. "kubectl get managed"
	if gvk.Kind == CustomResourceDefinitionKind && item.Category != "" {
		gvks, err := s.lookup.LookupCrdsByCategory(ctx, item.Category)
		if err != nil {
			return 0, fmt.Errorf("failed to lookup CRDs by category %s: %w", item.Category, err)
		}

		if len(gvks) == 0 {
			return 0, fmt.Errorf("no CRDs found for category %s", item.Category)
		}

		count := 0
		errs := []error{}
		for _, gvk := range gvks {
			c, err := s.DeleteResources(ctx, dryRun, gvk, item.Namespace)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to cleanup resources of kind %s in namespace %s: %w", gvk.Kind, item.Namespace, err))
				continue
			}
			count += c
		}
		return count, errors.Join(errs...)
	}

	// if no name was specified, delete all resources of the specified kind, optionally scoped to a namespace
	if item.Name == "" {
		c, err := s.DeleteResources(ctx, dryRun, gvk, item.Namespace)
		if err != nil {
			return c, fmt.Errorf("failed to cleanup resources of kind %s in namespace %s: %w", item.Kind, item.Namespace, err)
		}
		return c, nil
	}

	// if a name was specified, delete the specific resource
	c, err := s.DeleteNamedResource(ctx, dryRun, gvk, item.Namespace, item.Name)
	if err != nil {
		return c, fmt.Errorf("failed to cleanup named resource %s/%s: %w", item.Namespace, item.Name, err)
	}
	return c, nil
}

// DeleteResources deletes all resources of a specific kind in a given namespace.
// It returns the count of deleted resources and any errors encountered during deletion.
// If dryRun is true, it only logs the resources that would be deleted without actually deleting them.
func (s *DeleteService) DeleteResources(ctx context.Context, dryRun bool, gvk schema.GroupVersionKind, ns string) (int, error) {
	list, err := s.lookup.ListResources(ctx, gvk, ns)
	if err != nil {
		return 0, fmt.Errorf("failed to list resources of kind %s in namespace %s: %w", gvk.Kind, ns, err)
	}

	if len(list.Items) == 0 {
		s.logger.Info("No resources found to delete", "kind", gvk.Kind, "namespace", ns)
		return 0, nil // Nothing to delete
	}

	if dryRun {
		s.logger.Info("Dry run mode, skipping deletion", "kind", gvk.Kind, "namespace", ns, "count", len(list.Items))
		for _, item := range list.Items {
			s.logger.Info("Would delete item", "kind", gvk.Kind, "namespace", item.GetNamespace(), "name", item.GetName())
		}
		return len(list.Items), nil
	}

	count := 0
	errs := []error{}
	for _, item := range list.Items {
		s.logger.Info("Deleting item", "kind", gvk.Kind, "namespace", item.GetNamespace(), "name", item.GetName())
		if err := s.client.Delete(ctx, &item); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete %s/%s: %w", item.GetNamespace(), item.GetName(), err))
			continue
		}
		count++
	}

	return count, errors.Join(errs...)
}

// DeleteNamedResource deletes a specific resource by its kind, namespace, and name.
// It returns the count of deleted resources (1 if successful, 0 if not found) and any errors encountered during deletion.
// If dryRun is true, it only logs the resource that would be deleted without actually deleting it.
func (s *DeleteService) DeleteNamedResource(ctx context.Context, dryRun bool, gvk schema.GroupVersionKind, ns string, name string) (int, error) {
	item := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       gvk.Kind,
			APIVersion: gvk.GroupVersion().String(),
		},
	}

	if err := s.client.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, item); err != nil {
		return 0, fmt.Errorf("failed to get %s/%s: %w", ns, name, err)
	}

	if dryRun {
		logger := log.FromContext(ctx)
		logger.Info("Dry run mode, skipping deletion", "kind", gvk.Kind, "namespace", ns, "name", name)
		logger.Info("Would delete item", "kind", gvk.Kind, "namespace", item.GetNamespace(), "name", item.GetName())
		return 1, nil
	}

	s.logger.Info("Deleting item", "kind", gvk.Kind, "namespace", item.GetNamespace(), "name", item.GetName())
	if err := s.client.Delete(ctx, item); err != nil {
		return 0, fmt.Errorf("failed to delete %s/%s: %w", item.GetNamespace(), item.GetName(), err)
	}

	return 1, nil
}
