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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cleanupv1alpha1 "github.com/MetroStar/quartz-operator/api/v1alpha1"
	"github.com/MetroStar/quartz-operator/internal/services"
)

const (
	ConditionComplete           = "Complete"
	ConditionInitialized        = "Initialized"
	ReasonCompletedSuccessfully = "CompletedSuccessfully"
	ReasonCompletedWithErrors   = "CompletedWithErrors"
	ReasonNoResources           = "NoResources"
	ReasonReconciling           = "Reconciling"
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
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=update;patch
// +kubebuilder:rbac:groups=*,resources=*,verbs=delete;list;get;watch

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

	update := services.NewUpdateService(r.Client)

	if len(obj.Status.Conditions) == 0 {
		// Initialize conditions if not set
		if err := update.UpdateCondition(ctx, obj, ConditionInitialized, ReasonReconciling, "Reconciliation started"); err != nil {
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
		logger.Info("No resources specified, skipping")
		if err := update.UpdateCondition(ctx, obj, ConditionComplete, ReasonNoResources, "No resources specified for processing"); err != nil {
			logger.Error(err, "failed to update PreClusterDestroyCleanup status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	cleanup := services.NewCleanupService(ctx, r.Client, r.Config)
	count, err := cleanup.CleanupItems(ctx, obj.Spec.DryRun, items)
	if err != nil {
		logger.Error(err, "Error(s) occurred during processing")
		if err := update.UpdateCondition(ctx, obj, ConditionComplete, ReasonCompletedWithErrors, fmt.Sprintf("Processed %d resources with error(s): %v", count, err)); err != nil {
			logger.Error(err, "failed to update PreClusterDestroyCleanup status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	if err := update.UpdateCondition(ctx, obj, ConditionComplete, ReasonCompletedSuccessfully, fmt.Sprintf("Processed %d resources", count)); err != nil {
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
