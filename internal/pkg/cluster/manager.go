package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/jbetancur/dashboard/internal/pkg/providers"
	"k8s.io/client-go/kubernetes"
)

// Manager handles multiple Kubernetes cluster connections
type Manager struct {
	connections map[string]*Connection
	mu          sync.RWMutex
	logger      *slog.Logger
	provider    providers.Provider
	ctx         context.Context
}

// ClusterInfo represents summary information about a cluster
type ClusterInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	ApiURL string `json:"apiUrl"`
	Status string `json:"status"`
}

// NewManager creates a new ClusterManager
func NewManager(ctx context.Context, logger *slog.Logger, provider providers.Provider) *Manager {
	return &Manager{
		connections: make(map[string]*Connection),
		logger:      logger,
		provider:    provider,
		ctx:         ctx,
	}
}

// Register adds a new cluster to the ClusterManager
func (m *Manager) Register(clusterName, apiURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.connections == nil {
		m.connections = make(map[string]*Connection)
	}

	if _, exists := m.connections[clusterName]; exists {
		m.logger.Warn("Cluster already exists", "clusterName", clusterName)
		return nil
	}

	m.connections[clusterName] = &Connection{
		ID: clusterName,
		// Additional fields will be initialized when GetCluster is called
	}

	m.logger.Info("Cluster registered", "clusterName", clusterName, "apiURL", apiURL)
	return nil
}

// GetCluster retrieves or initializes a cluster connection
func (m *Manager) GetCluster(clusterID string) (*Connection, error) {
	m.mu.RLock()
	cluster, exists := m.connections[clusterID]
	m.mu.RUnlock()

	if exists && cluster.IsConnected() {
		return cluster, nil
	}

	// Authenticate and initialize the cluster connection
	m.logger.Info("Authenticating cluster", "clusterID", clusterID)
	restConfig, err := m.provider.Authenticate(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate cluster %s: %w", clusterID, err)
	}

	// Create Kubernetes client
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client for cluster %s: %w", clusterID, err)
	}

	// Initialize the connection
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check again in case another goroutine initialized it
	if existing, exists := m.connections[clusterID]; exists && existing.IsConnected() {
		return existing, nil
	}

	// Create or update the connection
	if cluster == nil {
		cluster = NewConnection(clusterID, client, restConfig)
		m.connections[clusterID] = cluster
	} else {
		cluster.Client = client
		cluster.AuthDone = true
	}

	// Initialize informers
	cluster.InitializeInformers()

	// Monitor context for cancellation
	go func() {
		<-m.ctx.Done()
		m.mu.Lock()
		defer m.mu.Unlock()
		if conn, exists := m.connections[clusterID]; exists {
			conn.Stop()
		}
	}()

	m.logger.Info("Cluster connection initialized", "clusterID", clusterID)
	return cluster, nil
}

// StopCluster stops a specific cluster connection and its informers
func (m *Manager) StopCluster(clusterID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, exists := m.connections[clusterID]
	if !exists {
		return fmt.Errorf("cluster %s not found", clusterID)
	}

	cluster.Stop()
	delete(m.connections, clusterID)
	m.logger.Info("Cluster connection stopped", "clusterID", clusterID)
	return nil
}

// StopAllClusters stops all cluster connections and their informers
func (m *Manager) StopAllClusters() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for clusterID, cluster := range m.connections {
		cluster.Stop()
		m.logger.Info("Cluster connection stopped", "clusterID", clusterID)
		delete(m.connections, clusterID)
	}
}

// ListClusters returns information about all registered clusters
func (m *Manager) ListClusters() []ClusterInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clusters := make([]ClusterInfo, 0, len(m.connections))
	for clusterID, conn := range m.connections {
		apiUrl := ""
		if conn.Config != nil {
			apiUrl = conn.Config.Host
		}

		clusters = append(clusters, ClusterInfo{
			ID:     clusterID,
			Name:   clusterID, // Using ID as name unless you have custom names stored
			ApiURL: apiUrl,
		})
	}

	return clusters
}

// GetConnections returns a copy of all cluster connections
func (m *Manager) GetConnections() map[string]*Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	connections := make(map[string]*Connection, len(m.connections))
	for id, conn := range m.connections {
		connections[id] = conn
	}

	return connections
}

// CheckClusterHealth checks the health of all registered clusters
func (m *Manager) CheckClusterHealth() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health := make(map[string]bool, len(m.connections))
	for id, conn := range m.connections {
		if status, err := conn.GetHealthStatus(); err == nil {
			health[id] = status
		} else {
			health[id] = false
			m.logger.Warn("Cluster health check failed",
				"clusterID", id,
				"error", err)
		}
	}

	return health
}
