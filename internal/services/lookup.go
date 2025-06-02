package services

import (
	"context"
	"fmt"
	"slices"
	"strings"

	apiextclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LookupService provides methods to look up GroupVersionKind and CustomResourceDefinitions (CRDs).
type LookupService struct {
	client client.Client
	config *rest.Config
}

// NewLookupService creates a new LookupService instance.
func NewLookupService(client client.Client, config *rest.Config) *LookupService {
	return &LookupService{
		client: client,
		config: config,
	}
}

// LookupGroupKind attempts to find the GroupVersionKind for a given kind.
// It first tries to find it directly by kind, and if that fails, it tries to find it by group and kind.
func (s *LookupService) LookupGroupKind(kind string) (schema.GroupVersionKind, error) {
	mapper := s.client.RESTMapper()

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

// LookupCrdsByCategory looks up CustomResourceDefinitions (CRDs) by their category.
// It returns a slice of GroupVersionKind for CRDs that match the specified category.
func (s *LookupService) LookupCrdsByCategory(ctx context.Context, category string) ([]schema.GroupVersionKind, error) {
	crdClient, err := apiextclient.NewForConfig(s.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create CRD client: %w", err)
	}

	crds, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list CRDs: %w", err)
	}

	gvks := []schema.GroupVersionKind{}
	for _, crd := range crds.Items {
		if slices.ContainsFunc(crd.Spec.Names.Categories, func(c string) bool {
			return strings.EqualFold(c, category)
		}) {
			gvks = append(gvks, schema.GroupVersionKind{
				Group:   crd.Spec.Group,
				Version: crd.Spec.Versions[0].Name, // Use the first version for simplicity
				Kind:    crd.Spec.Names.Kind,
			})
		}
	}

	return gvks, nil
}

// ListResources lists all resources of a specific GroupVersionKind in a given namespace.
// It returns a PartialObjectMetadataList containing the resources found.
// If the GroupVersionKind does not specify a version, it defaults to "v1".
func (s *LookupService) ListResources(ctx context.Context, gvk schema.GroupVersionKind, ns string) (*metav1.PartialObjectMetadataList, error) {
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

	if err := s.client.List(ctx, list, opts...); err != nil {
		return nil, fmt.Errorf("failed to list %s in namespace %s: %w", gvk.Kind, ns, err)
	}

	return list, nil
}
