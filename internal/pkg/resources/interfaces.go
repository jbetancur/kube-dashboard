package resources

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ResourceManager is the base interface for all resource managers
type ResourceManager interface {
	// StartInformer starts the resource informer
	StartInformer(ctx context.Context) error

	// Stop stops the resource informer
	Stop()
}

// ResourceProvider is the base interface for all resource providers
type ResourceProvider interface {
	// GetResourceType returns the resource type name
	GetResourceType() string
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

// ObjectMeta extracts common metadata from any Kubernetes resource
func ObjectMeta(obj interface{}) metav1.ObjectMeta {
	if obj, ok := obj.(metav1.Object); ok {
		return metav1.ObjectMeta{
			Name:              obj.GetName(),
			Namespace:         obj.GetNamespace(),
			CreationTimestamp: obj.GetCreationTimestamp(),
			Labels:            obj.GetLabels(),
			Annotations:       obj.GetAnnotations(),
		}
	}
	return metav1.ObjectMeta{}
}
