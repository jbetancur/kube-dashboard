package configmaps

import (
	"context"
	"fmt"

	"github.com/jbetancur/dashboard/internal/pkg/cluster"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigMapProvider implements ConfigMapProvider for multiple clusters
type ConfigMapProvider struct {
	clusterManager *cluster.Manager
}

// NewConfigMapProvider creates a new provider
func NewConfigMapProvider(clusterManager *cluster.Manager) *ConfigMapProvider {
	return &ConfigMapProvider{
		clusterManager: clusterManager,
	}
}

// ListConfigMaps lists config maps in a specific namespace from a specific cluster
func (p *ConfigMapProvider) ListConfigMaps(ctx context.Context, clusterID, namespace string) ([]v1.ConfigMap, error) {
	// Get the cluster connection
	cluster, err := p.clusterManager.GetCluster(clusterID)
	if err != nil {
		return nil, fmt.Errorf("cluster not found: %w", err)
	}

	// Use the Kubernetes API directly (no informers)
	configMapList, err := cluster.Client.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list config maps: %w", err)
	}

	return configMapList.Items, nil
}

// GetConfigMap gets a specific config map from a specific cluster and namespace
func (p *ConfigMapProvider) GetConfigMap(ctx context.Context, clusterID, namespace, configMapName string) (*v1.ConfigMap, error) {
	// Get the cluster connection
	cluster, err := p.clusterManager.GetCluster(clusterID)
	if err != nil {
		return nil, fmt.Errorf("cluster not found: %w", err)
	}

	// Use the Kubernetes API directly (no informers)
	configMap, err := cluster.Client.CoreV1().ConfigMaps(namespace).Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get config map: %w", err)
	}

	return configMap.DeepCopy(), nil
}
