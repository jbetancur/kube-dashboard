package core

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jbetancur/dashboard/internal/pkg/providers"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

// ClusterGetter defines the interface for retrieving cluster information
type ClusterGetter interface {
	GetClusterConnection(id string) (*ClusterConnection, error)
}

// ClusterManager handles multiple Kubernetes cluster connections
type ClusterManager struct {
	Clusters sync.Map // Replaces map[string]*ClusterConnection and sync.RWMutex
	logger   *slog.Logger
	provider providers.Provider
}

// ClusterConnection represents a Kubernetes cluster connection
type ClusterConnection struct {
	ID       string
	Client   *kubernetes.Clientset
	Informer informers.SharedInformerFactory
	StopCh   chan struct{}
	AuthDone bool // Tracks whether authentication has been completed

}

// NewClusterManager creates a new cluster manager
func NewClusterManager(logger *slog.Logger, provider providers.Provider) *ClusterManager {
	return &ClusterManager{
		logger:   logger,
		provider: provider,
	}
}

// AddCluster adds a new cluster to the manager
func (cm *ClusterManager) RegisterCluster(id string) error {
	// Check if the cluster already exists
	if _, exists := cm.GetClusterConnection(id); exists == nil {
		return fmt.Errorf("cluster %s already exists", id)
	}

	// Store the cluster metadata without authenticating
	clusterConnection := &ClusterConnection{
		ID:     id,
		StopCh: make(chan struct{}),
	}

	cm.Clusters.Store(id, clusterConnection)
	cm.logger.Info("Cluster registered successfully (lazy authentication)", "clusterID", id)

	return nil
}

func (cm *ClusterManager) GetClusterConnection(clusterID string) (*ClusterConnection, error) {
	// Check if the cluster is already initialized
	value, exists := cm.Clusters.Load(clusterID)
	if !exists {
		return nil, fmt.Errorf("cluster %s is not registered", clusterID)
	}

	cluster, ok := value.(*ClusterConnection)
	if !ok || cluster == nil {
		return nil, fmt.Errorf("cluster %s connection is invalid or nil", clusterID)
	}

	// Perform lazy authentication if the cluster is not yet authenticated
	if !cluster.AuthDone {
		cm.logger.Info("Authenticating cluster (lazy)", "clusterID", clusterID)
		restConfig, err := cm.provider.Authenticate(clusterID)
		if err != nil {
			return nil, fmt.Errorf("failed to authenticate cluster %s: %w", clusterID, err)
		}

		// Create Kubernetes client
		client, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Kubernetes client for cluster %s: %w", clusterID, err)
		}

		// Create informer factory
		informer := informers.NewSharedInformerFactory(client, time.Minute*5)

		// Update the cluster connection
		cluster.Client = client
		cluster.Informer = informer
		cluster.AuthDone = true
	}

	return cluster, nil
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

func StartInformers(clusters []providers.ClusterConfig, startInformer func(clusterID string) error) error {
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

// ListClusters returns all registered cluster IDs
func (cm *ClusterManager) ListClusters() []string {
	clusterIDs := []string{}

	cm.Clusters.Range(func(key, value interface{}) bool {
		clusterIDs = append(clusterIDs, key.(string))
		return true
	})

	return clusterIDs
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

func (cm *ClusterManager) WithReauthentication(clusterID string, apiCall func(client *kubernetes.Clientset) error) error {
	// Get the cluster connection
	cluster, err := cm.GetClusterConnection(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	// Attempt the API call
	err = apiCall(cluster.Client)
	if err != nil {
		// Check if the error is an authentication error
		if isAuthenticationError(err) {
			cm.logger.Warn("Authentication error detected, re-authenticating", "clusterID", clusterID)

			// Re-authenticate the cluster
			restConfig, authErr := cm.provider.Authenticate(clusterID)
			if authErr != nil {
				return fmt.Errorf("failed to re-authenticate cluster: %w", authErr)
			}

			// Reinitialize the cluster connection
			client, clientErr := kubernetes.NewForConfig(restConfig)
			if clientErr != nil {
				return fmt.Errorf("failed to create Kubernetes client after re-authentication: %w", clientErr)
			}

			// Update the cluster connection
			cluster.Client = client
			cluster.Informer = informers.NewSharedInformerFactory(client, time.Minute*5)

			// Retry the API call
			return apiCall(client)
		}

		// Return the original error if it's not an authentication error
		return err
	}

	return nil
}

// Helper function to detect authentication errors
func isAuthenticationError(err error) bool {
	// Check for specific error types or messages (e.g., 401 Unauthorized)
	return err != nil && (strings.Contains(err.Error(), "Unauthorized") || strings.Contains(err.Error(), "expired"))
}
