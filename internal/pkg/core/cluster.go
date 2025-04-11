package core

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jbetancur/dashboard/internal/pkg/messaging"
	"github.com/jbetancur/dashboard/internal/pkg/providers"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

// ClusterConnectionPayload represents the payload sent to the REST API
type ClusterConnectionPayload struct {
	ClusterName string `json:"clusterName"`
	APIURL      string `json:"apiURL"`
}

// ClusterManager handles multiple Kubernetes cluster connections
type ClusterManager struct {
	Connections map[string]*ClusterConnection
	mu          sync.RWMutex
	logger      *slog.Logger
	provider    providers.Provider
}

// ClusterConnection represents a Kubernetes cluster connection
type ClusterConnection struct {
	ID       string
	Client   *kubernetes.Clientset
	Informer informers.SharedInformerFactory
	StopCh   chan struct{}
	AuthDone bool
	Running  bool // Tracks whether informers are running
}

// NewClusterManager creates a new ClusterManager
func NewClusterManager(logger *slog.Logger, provider providers.Provider) *ClusterManager {
	return &ClusterManager{
		Connections: make(map[string]*ClusterConnection),
		logger:      logger,
		provider:    provider,
	}
}

// Register adds a new cluster to the ClusterManager.
func (cm *ClusterManager) Register(clusterName, apiURL string) error {
	if cm.Connections == nil {
		cm.Connections = make(map[string]*ClusterConnection)
	}

	if _, exists := cm.Connections[clusterName]; exists {
		return fmt.Errorf("cluster %s already exists", clusterName)
	}

	cm.Connections[clusterName] = &ClusterConnection{
		ID: clusterName,
		// Additional fields can be initialized here if needed
	}

	cm.logger.Info("Cluster added", "clusterName", clusterName, "apiURL", apiURL)

	return nil
}

// GetCluster retrieves or initializes a cluster connection
func (cm *ClusterManager) GetCluster(clusterID string) (*ClusterConnection, error) {
	cm.mu.RLock()
	cluster, exists := cm.Connections[clusterID]
	cm.mu.RUnlock()

	if exists {
		// If the cluster is already authenticated, return it
		if cluster.AuthDone {
			return cluster, nil
		}
	}

	// Authenticate and initialize the cluster connection
	cm.logger.Info("Authenticating cluster", "clusterID", clusterID)
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

	// Initialize the cluster connection
	cluster = &ClusterConnection{
		ID:       clusterID,
		Client:   client,
		Informer: informer,
		StopCh:   make(chan struct{}),
		AuthDone: true,
		Running:  false,
	}

	// Cache the cluster connection
	cm.mu.Lock()
	cm.Connections[clusterID] = cluster
	cm.mu.Unlock()

	cm.logger.Info("Cluster connection initialized", "clusterID", clusterID)
	return cluster, nil
}

// StopCluster stops a specific cluster connection and its informers
func (cm *ClusterManager) StopCluster(clusterID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cluster, exists := cm.Connections[clusterID]
	if !exists {
		return fmt.Errorf("cluster %s not found", clusterID)
	}

	close(cluster.StopCh)
	delete(cm.Connections, clusterID)
	cm.logger.Info("Cluster connection stopped", "clusterID", clusterID)
	return nil
}

// StopAllClusters stops all cluster connections and their informers
func (cm *ClusterManager) StopAllClusters() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for clusterID, cluster := range cm.Connections {
		close(cluster.StopCh)
		cm.logger.Info("Cluster connection stopped", "clusterID", clusterID)
		delete(cm.Connections, clusterID)
	}
}

// ListClusters returns a list of all registered cluster IDs
func (cm *ClusterManager) ListClusters() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	clusterIDs := make([]string, 0, len(cm.Connections))
	for clusterID := range cm.Connections {
		clusterIDs = append(clusterIDs, clusterID)
	}

	return clusterIDs
}

// PublishCluster sends the cluster connection details via the message queue
func PublishCluster(messageQueue *messaging.GRPCClient, clusterName, apiServerURL string, logger *slog.Logger) error {
	payload := ClusterConnectionPayload{
		ClusterName: clusterName,
		APIURL:      apiServerURL,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal cluster connection payload: %w", err)
	}

	// Retry logic
	for i := 0; i < 5; i++ {
		err = messageQueue.Publish("cluster_registered", data)
		if err == nil {
			return nil
		}

		logger.Warn("Failed to publish cluster connection, retrying...", "attempt", i+1, "error", err)
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("failed to publish cluster connection after retries: %w", err)
}
