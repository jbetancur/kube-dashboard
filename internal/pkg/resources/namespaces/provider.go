package namespaces

import (
	"context"
	"fmt"

	"github.com/jbetancur/dashboard/internal/pkg/cluster"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MultiClusterNamespaceProvider implements NamespaceProvider for multiple clusters
type MultiClusterNamespaceProvider struct {
	clusterManager *cluster.Manager
}

// NewMultiClusterNamespaceProvider creates a new provider
func NewMultiClusterNamespaceProvider(clusterManager *cluster.Manager) *MultiClusterNamespaceProvider {
	return &MultiClusterNamespaceProvider{
		clusterManager: clusterManager,
	}
}

// ListNamespaces lists namespaces from a specific cluster
func (p *MultiClusterNamespaceProvider) ListNamespaces(ctx context.Context, clusterID string) ([]v1.Namespace, error) {
	// Get the cluster connection
	cluster, err := p.clusterManager.GetCluster(clusterID)
	if err != nil {
		return nil, fmt.Errorf("cluster not found: %w", err)
	}

	// Use the Kubernetes API directly (no informers)
	namespaceList, err := cluster.Client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	return namespaceList.Items, nil
}

// GetNamespace gets a specific namespace from a specific cluster
func (p *MultiClusterNamespaceProvider) GetNamespace(ctx context.Context, clusterID, namespaceName string) (*v1.Namespace, error) {
	// Get the cluster connection
	cluster, err := p.clusterManager.GetCluster(clusterID)
	if err != nil {
		return nil, fmt.Errorf("cluster not found: %w", err)
	}

	// Use the Kubernetes API directly (no informers)
	namespace, err := cluster.Client.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace: %w", err)
	}

	return namespace, nil
}
