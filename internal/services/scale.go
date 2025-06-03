package services

import (
	"context"
	"errors"
	"fmt"

	cleanupv1alpha1 "github.com/MetroStar/quartz-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ScaleService provides methods to scale resources like Deployments and StatefulSets.
type ScaleService struct {
	client client.Client
	lookup *LookupService
	logger logr.Logger
}

// NewScaleService creates a new ScaleService instance.
func NewScaleService(ctx context.Context, client client.Client, lookup *LookupService) *ScaleService {
	return &ScaleService{
		client: client,
		lookup: lookup,
		logger: log.FromContext(ctx),
	}
}

// ScaleItem scales a resource to specified replicas if it is a Deployment or StatefulSet.
// It returns the count of scaled resources (1 if successful, 0 if not applicable) and any errors encountered during scaling.
// If dryRun is true, it only logs the action without actually scaling the resource.
func (s *ScaleService) ScaleItem(ctx context.Context, dryRun bool, gvk schema.GroupVersionKind, item cleanupv1alpha1.PreClusterDestroyCleanupItem, replicas *int32) (int, error) {
	if gvk.Kind != DeploymentKind && gvk.Kind != StatefulSetKind {
		return 0, fmt.Errorf("scaling is not supported for kind %s", gvk.Kind)
	}

	if item.Name != "" {
		c, err := s.ScaleKind(ctx, dryRun, gvk, item.Namespace, item.Name, replicas)
		if err != nil {
			return 0, fmt.Errorf("failed to scale %s/%s: %w", item.Namespace, item.Name, err)
		}
		return c, nil
	}

	// lookup all resources of the specified kind in the namespace
	list, err := s.lookup.ListResources(ctx, gvk, item.Namespace)
	if err != nil {
		return 0, fmt.Errorf("failed to list resources of kind %s in namespace %s: %w", gvk.Kind, item.Namespace, err)
	}

	if len(list.Items) == 0 {
		s.logger.Info("No resources found to scale", "kind", gvk.Kind, "namespace", item.Namespace)
		return 0, nil // Nothing to scale
	}

	count := 0
	errs := []error{}
	for _, i := range list.Items {
		c, err := s.ScaleKind(ctx, dryRun, gvk, i.GetNamespace(), i.GetName(), replicas)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to scale %s/%s: %w", i.GetNamespace(), i.GetName(), err))
		}
		count += c
	}

	return count, errors.Join(errs...)
}

func (r *ScaleService) ScaleKind(ctx context.Context, dryRun bool, gvk schema.GroupVersionKind, ns string, name string, replicas *int32) (int, error) {
	switch gvk.Kind {
	case DeploymentKind:
		return r.ScaleDeployment(ctx, dryRun, ns, name, replicas)
	case StatefulSetKind:
		return r.ScaleStatefulSet(ctx, dryRun, ns, name, replicas)
	default:
		return 0, fmt.Errorf("replica scaling is not supported for kind %s", gvk.Kind)
	}
}

// ScaleDeployment scales a Deployment to specified replicas.
// It returns the count of scaled resources (1 if successful, 0 if not applicable) and any errors encountered during scaling.
// If dryRun is true, it only logs the action without actually scaling the resource.
func (s *ScaleService) ScaleDeployment(ctx context.Context, dryRun bool, ns string, name string, replicas *int32) (int, error) {
	if name == "" {
		s.logger.Info("No name specified for scaling, skipping")
		return 0, nil // Nothing to scale
	}

	if dryRun {
		s.logger.Info("Dry run mode, skipping scaling", "kind", DeploymentKind, "namespace", ns, "name", name, "replicas", *replicas)
		return 1, nil
	}

	s.logger.Info("Scaling deployment", "kind", DeploymentKind, "namespace", ns, "name", name, "replicas", *replicas)
	deployment := &appsv1.Deployment{}
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, deployment); err != nil {
		return 0, fmt.Errorf("failed to get %s/%s: %w", ns, name, err)
	}

	deployment.Spec.Replicas = replicas
	if err := s.client.Update(ctx, deployment); err != nil {
		return 0, fmt.Errorf("failed to scale %s/%s: %w", ns, name, err)
	}

	s.logger.Info("Scaled deployment", "kind", DeploymentKind, "namespace", ns, "name", name, "replicas", *replicas)
	return 1, nil // Indicate that we scaled
}

// ScaleStatefulSet scales a StatefulSet to specified replicas.
func (s *ScaleService) ScaleStatefulSet(ctx context.Context, dryRun bool, ns string, name string, replicas *int32) (int, error) {
	if name == "" {
		s.logger.Info("No name specified for scaling, skipping")
		return 0, nil // Nothing to scale
	}

	if dryRun {
		s.logger.Info("Dry run mode, skipping scaling", "kind", StatefulSetKind, "namespace", ns, "name", name, "replicas", *replicas)
		return 1, nil
	}

	s.logger.Info("Scaling statefulset", "kind", StatefulSetKind, "namespace", ns, "name", name, "replicas", *replicas)
	statefulSet := &appsv1.StatefulSet{}
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, statefulSet); err != nil {
		return 0, fmt.Errorf("failed to get %s/%s: %w", ns, name, err)
	}

	statefulSet.Spec.Replicas = replicas
	if err := s.client.Update(ctx, statefulSet); err != nil {
		return 0, fmt.Errorf("failed to scale %s/%s: %w", ns, name, err)
	}

	s.logger.Info("Scaled statefulset", "kind", StatefulSetKind, "namespace", ns, "name", name, "replicas", *replicas)
	return 1, nil
}
