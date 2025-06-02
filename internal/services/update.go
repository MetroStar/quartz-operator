package services

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cleanupv1alpha1 "github.com/MetroStar/quartz-operator/api/v1alpha1"
)

// UpdateService provides methods to update the status of PreClusterDestroyCleanup objects.
type UpdateService struct {
	client client.Client
}

// NewUpdateService creates a new UpdateService instance.
func NewUpdateService(client client.Client) *UpdateService {
	return &UpdateService{
		client: client,
	}
}

// UpdateCondition updates the status condition of a PreClusterDestroyCleanup object.
// It sets the condition type, status, reason, and message, and updates the object status in the cluster.
// If the update fails, it returns an error.
func (s *UpdateService) UpdateCondition(ctx context.Context, obj *cleanupv1alpha1.PreClusterDestroyCleanup, t string, reason string, message string) error {
	meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type:    t,
		Status:  metav1.ConditionTrue,
		Reason:  reason,
		Message: message,
	})
	if err := s.client.Status().Update(ctx, obj); err != nil {
		return fmt.Errorf("failed to update PreClusterDestroyCleanup status: %w", err)
	}

	return nil
}
