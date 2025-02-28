package core

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ClusterManager handles multiple Kubernetes cluster connections
type ClusterManager struct {
	Clusters sync.Map // Replaces map[string]*ClusterConnection and sync.RWMutex
	logger   *slog.Logger
}

// ClusterConnection represents a Kubernetes cluster connection
type ClusterConnection struct {
	ID       string
	Client   *kubernetes.Clientset
	Informer informers.SharedInformerFactory
	StopCh   chan struct{}
}

// NewClusterManager creates a new cluster manager
func NewClusterManager(logger *slog.Logger) *ClusterManager {
	return &ClusterManager{
		logger: logger,
	}
}

// AddCluster adds a new cluster to the manager
func (cm *ClusterManager) AddCluster(id, kubeconfigPath string) error {
	// Check if cluster already exists
	if _, exists := cm.GetCluster(id); exists == nil {
		return fmt.Errorf("cluster %s already exists", id)
	}

	// Create Kubernetes client config with the specified context
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: id,
	}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to create REST config for context %s: %w", id, err)
	}

	// Create Kubernetes client
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Create informer factory
	stopCh := make(chan struct{})
	informer := informers.NewSharedInformerFactory(client, time.Minute*5)

	// Store cluster
	clusterConnection := &ClusterConnection{
		ID:       id,
		Client:   client,
		Informer: informer,
		StopCh:   stopCh,
	}

	cm.Clusters.Store(id, clusterConnection)

	// Start informers
	go informer.Start(stopCh)

	cm.logger.Info("Cluster added successfully", "clusterID", id, "context", id)

	return nil
}

// RemoveCluster removes a cluster and cleans up its resources
func (cm *ClusterManager) RemoveCluster(id string) error {
	value, exists := cm.GetCluster(id)
	if exists != nil {
		return fmt.Errorf("cluster %s not found", id)
	}

	cluster := value
	// Stop informers
	close(cluster.StopCh)

	// Remove from map
	cm.Clusters.Delete(id)

	cm.logger.Info("Cluster removed successfully", "clusterID", id)

	return nil
}

// GetCluster returns a cluster by ID
func (cm *ClusterManager) GetCluster(id string) (*ClusterConnection, error) {
	value, exists := cm.Clusters.Load(id)
	if !exists {
		return nil, fmt.Errorf("cluster %s not found", id)
	}

	return value.(*ClusterConnection), nil
}

// ListClusters returns all registered cluster IDs
func (cm *ClusterManager) ListClusters() []string {
	clusterIDs := []string{}

	cm.Clusters.Range(func(key, value interface{}) bool {
		clusterIDs = append(clusterIDs, key.(string))
		return true
	})

	return clusterIDs
}

// StopAllClusters stops all clusters and clears the map
func (cm *ClusterManager) StopAllClusters() {
	cm.Clusters.Range(func(key, value interface{}) bool {
		cluster := value.(*ClusterConnection)
		close(cluster.StopCh)

		cm.logger.Info("Cluster stopped", "clusterID", cluster.ID)

		cm.Clusters.Delete(key)

		return true
	})
}

// StartInformers is a generic function to start informers for multiple clusters concurrently
func StartInformers(clusters []ClusterConfig, startInformer func(clusterID string) error) error {
	errCh := make(chan error, len(clusters))
	var wg sync.WaitGroup

	// Start informers for all clusters concurrently
	for _, cluster := range clusters {
		wg.Add(1)
		go func(clusterID string) {
			defer wg.Done()
			if err := startInformer(clusterID); err != nil {
				errCh <- fmt.Errorf("error starting informer for cluster %s: %w", clusterID, err)
			}
		}(cluster.ID)
	}

	// Wait for all informers to start
	wg.Wait()
	close(errCh)

	// Collect errors
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to start informers: %v", errs)
	}

	return nil
}

// ListResources is a generic function to list resources using a lister
func ListResources[T any](ctx context.Context, clusterID string, lister func(labels.Selector) ([]T, error)) ([]T, error) {
	resources, err := lister(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list resources for cluster %s: %w", clusterID, err)
	}

	// Sort resources if they have a Name field
	sort.Slice(resources, func(i, j int) bool {
		return fmt.Sprintf("%v", resources[i]) < fmt.Sprintf("%v", resources[j])
	})

	return resources, nil
}

// GetResource is a generic function to get a specific resource using a lister
func GetResource[T any](ctx context.Context, clusterID, resourceName string, getter func(string) (T, error)) (T, error) {
	resource, err := getter(resourceName)
	if err != nil {
		return resource, fmt.Errorf("failed to get resource %s for cluster %s: %w", resourceName, clusterID, err)
	}

	return resource, nil
}
