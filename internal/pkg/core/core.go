package core

import (
	"context"
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/labels"
)

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
