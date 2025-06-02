package services

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cleanupv1alpha1 "github.com/MetroStar/quartz-operator/api/v1alpha1"
	"github.com/go-logr/logr"
)

// CleanupService orchestrates the cleanup actions for PreClusterDestroyCleanupItems.
type CleanupService struct {
	lookup *LookupService
	scale  *ScaleService
	delete *DeleteService
	logger logr.Logger
}

// NewCleanupService creates a new CleanupService instance.
func NewCleanupService(ctx context.Context, client client.Client, config *rest.Config) *CleanupService {
	lookup := NewLookupService(client, config)
	return &CleanupService{
		lookup: lookup,
		scale:  NewScaleService(ctx, client, lookup),
		delete: NewDeleteService(ctx, client, lookup),
		logger: log.FromContext(ctx),
	}
}

// CleanupItems processes a list of PreClusterDestroyCleanupItems.
// It performs the specified action (scale to zero or delete) on each item.
// It returns the count of successfully processed items and any errors encountered.
// If dryRun is true, it simulates the actions without making actual changes.
func (s *CleanupService) CleanupItems(ctx context.Context, dryRun bool, items []cleanupv1alpha1.PreClusterDestroyCleanupItem) (int, error) {
	count := 0
	errs := []error{}
	for _, item := range items {
		if item.Kind == "" {
			errs = append(errs, fmt.Errorf("kind must be specified for item: %v", item))
			continue
		}

		gvk, err := s.lookup.LookupGroupKind(item.Kind)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to lookup group and kind for %s: %w", item.Kind, err))
			continue
		}

		switch item.Action {
		case cleanupv1alpha1.ActionScaleToZero:
			s.logger.Info("Scaling to zero", "kind", gvk.Kind, "namespace", item.Namespace, "name", item.Name)
			replicas := int32(0)
			c, err := s.scale.ScaleItem(ctx, dryRun, gvk, item, &replicas)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to scale %s/%s to zero: %w", item.Namespace, item.Name, err))
			}
			count += c
		case cleanupv1alpha1.ActionDelete:
			s.logger.Info("Deleting item", "kind", gvk.Kind, "namespace", item.Namespace, "name", item.Name)
			c, err := s.delete.DeleteItem(ctx, dryRun, gvk, item)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to delete %s/%s: %w", item.Namespace, item.Name, err))
			}
			count += c
		case cleanupv1alpha1.ActionUnknown:
			errs = append(errs, fmt.Errorf("action must be specified for item: %v", item))
		default:
			errs = append(errs, fmt.Errorf("unsupported action for item: %v", item))
		}
	}

	if len(errs) > 0 {
		return count, fmt.Errorf("%d errors occurred during processing: %w", len(errs), errors.Join(errs...))
	}

	return count, nil
}
