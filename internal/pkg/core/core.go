package core

import (
	"context"
	"fmt"
	"sort"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// NamespaceProvider defines the interface for namespace operations
type NamespaceProvider interface {
	ListNamespaces(ctx context.Context, clusterID string) ([]v1.Namespace, error)
	GetNamespace(ctx context.Context, clusterID, namespaceName string) (*v1.Namespace, error)
}

// PodProvider defines the interface for pod operations across clusters
type PodProvider interface {
	ListPods(ctx context.Context, clusterID, namespace string) ([]v1.Pod, error)
	GetPod(ctx context.Context, clusterID, namespace, podName string) (*v1.Pod, error)
}

// ListResources is a generic function to list resources using a lister
func ListResources[T any](ctx context.Context, lister func(labels.Selector) ([]T, error)) ([]T, error) {
	resources, err := lister(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	// Sort resources if they have a Name field
	sort.Slice(resources, func(i, j int) bool {
		return fmt.Sprintf("%v", resources[i]) < fmt.Sprintf("%v", resources[j])
	})

	return resources, nil
}

// GetResource is a generic function to get a specific resource using a lister
func GetResource[T any](ctx context.Context, resourceName string, getter func(string) (T, error)) (T, error) {
	resource, err := getter(resourceName)
	if err != nil {
		return resource, fmt.Errorf("failed to get resource %s: %w", resourceName, err)
	}

	return resource, nil
}
