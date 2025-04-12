package client

import (
	"fmt"
	"log/slog"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ClusterConfig holds Kubernetes client configuration details
type ClusterConfig struct {
	Client     *kubernetes.Clientset
	Config     *rest.Config
	Kubeconfig string
	Cluster    string
}

// ClientManager manages Kubernetes clients for multiple clusters
type ClientManager struct {
	configs map[string]*ClusterConfig
	logger  *slog.Logger
	watcher *KubeConfigWatcher
	mu      sync.RWMutex
}

// NewClientManager creates a new client manager
func NewClientManager(logger *slog.Logger) (*ClientManager, error) {
	cm := &ClientManager{
		configs: make(map[string]*ClusterConfig),
		logger:  logger,
	}

	// Create watcher with callback
	watcher, err := NewKubeConfigWatcher(logger, cm.handleKubeConfigChange)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubeconfig watcher: %w", err)
	}
	cm.watcher = watcher

	// Initial client creation
	kubeConfig := watcher.GetConfig()
	if kubeConfig == nil {
		return nil, fmt.Errorf("failed to get initial kubeconfig")
	}

	// Create clients for all contexts
	err = cm.createClientsFromConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create initial clients: %w", err)
	}

	// Start the watcher
	watcher.Start()

	return cm, nil
}

// GetClients returns all cluster clients
func (cm *ClientManager) GetClients() []*ClusterConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	clients := make([]*ClusterConfig, 0, len(cm.configs))
	for _, cfg := range cm.configs {
		clients = append(clients, cfg)
	}
	return clients
}

// GetClient returns a specific cluster client
func (cm *ClientManager) GetClient(clusterName string) (*ClusterConfig, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	client, exists := cm.configs[clusterName]
	return client, exists
}

// Stop stops the client manager and releases resources
func (cm *ClientManager) Stop() {
	if cm.watcher != nil {
		cm.watcher.Stop()
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()
	// Clear clients
	cm.configs = make(map[string]*ClusterConfig)
}

// handleKubeConfigChange is called when the kubeconfig changes
func (cm *ClientManager) handleKubeConfigChange(config *KubeConfig) {
	cm.logger.Info("Kubeconfig changed, updating clients")

	err := cm.createClientsFromConfig(config)
	if err != nil {
		cm.logger.Error("Failed to update clients after kubeconfig change", "error", err)
	}
}

// createClientsFromConfig creates clients from a kubeconfig
func (cm *ClientManager) createClientsFromConfig(config *KubeConfig) error {
	// Handle in-cluster case
	if config.Path == "" {
		inClusterConfig, err := rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("failed to get in-cluster config: %w", err)
		}

		clientset, err := kubernetes.NewForConfig(inClusterConfig)
		if err != nil {
			return fmt.Errorf("failed to create in-cluster client: %w", err)
		}

		cm.mu.Lock()
		cm.configs["in-cluster"] = &ClusterConfig{
			Client:     clientset,
			Config:     inClusterConfig,
			Kubeconfig: "",
			Cluster:    "in-cluster",
		}
		cm.mu.Unlock()

		return nil
	}

	// Handle file-based kubeconfig
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Track contexts to detect removals
	seen := make(map[string]bool)

	// Create clients for each context
	for contextName := range config.Contexts {
		cm.logger.Info("Processing context", "name", contextName)

		// Build rest.Config for this context
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: config.Path},
			&clientcmd.ConfigOverrides{CurrentContext: contextName},
		)

		restConfig, err := clientConfig.ClientConfig()
		if err != nil {
			cm.logger.Warn("Failed to build client config", "context", contextName, "error", err)
			continue
		}

		// Create clientset
		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			cm.logger.Warn("Failed to create client", "context", contextName, "error", err)
			continue
		}

		// Save client
		cm.configs[contextName] = &ClusterConfig{
			Client:     clientset,
			Config:     restConfig,
			Kubeconfig: config.Path,
			Cluster:    contextName,
		}
		seen[contextName] = true
	}

	// Remove contexts that no longer exist
	for name := range cm.configs {
		if !seen[name] && name != "in-cluster" {
			delete(cm.configs, name)
			cm.logger.Info("Removed client for deleted context", "context", name)
		}
	}

	return nil
}

// CreateClient creates a new Kubernetes client for the specified context
func CreateClient(contextName, kubeconfigPath string) (*kubernetes.Clientset, *rest.Config, error) {
	var config *rest.Config
	var err error

	if contextName == "in-cluster" || kubeconfigPath == "" {
		// In-cluster configuration
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create in-cluster config: %w", err)
		}
	} else {
		// Out-of-cluster configuration
		config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
			&clientcmd.ConfigOverrides{CurrentContext: contextName},
		).ClientConfig()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to build client config: %w", err)
		}
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create client: %w", err)
	}

	return clientset, config, nil
}
