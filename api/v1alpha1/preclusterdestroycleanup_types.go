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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type PreClusterDestroyCleanupItem struct {
	Kind      string `json:"kind,omitempty"`      // Kind is the name of the kind.
	Namespace string `json:"namespace,omitempty"` // Optional: Namespace where the resource is located
	Name      string `json:"name,omitempty"`      // Optional: Name of the resource
	Category  string `json:"category,omitempty"`  // Category is the category of the resource, e.g., "networking", "storage", etc.

	// +kubebuilder:validation:Enum=delete;scaleToZero
	Action string `json:"action,omitempty"` // Action is the action to be taken on the resource, e.g., "delete", "detach", etc.
}

// PreClusterDestroyCleanupSpec defines the desired state of PreClusterDestroyCleanup.
type PreClusterDestroyCleanupSpec struct {
	DryRun    bool                           `json:"dryRun,omitempty"` // DryRun indicates whether the cleanup should be performed or just logged
	Resources []PreClusterDestroyCleanupItem `json:"resources,omitempty"`
}

// PreClusterDestroyCleanupStatus defines the observed state of PreClusterDestroyCleanup.
type PreClusterDestroyCleanupStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PreClusterDestroyCleanup is the Schema for the preclusterdestroycleanups API.
type PreClusterDestroyCleanup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PreClusterDestroyCleanupSpec   `json:"spec,omitempty"`
	Status PreClusterDestroyCleanupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PreClusterDestroyCleanupList contains a list of PreClusterDestroyCleanup.
type PreClusterDestroyCleanupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PreClusterDestroyCleanup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PreClusterDestroyCleanup{}, &PreClusterDestroyCleanupList{})
}
