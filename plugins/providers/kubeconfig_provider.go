package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jbetancur/dashboard/internal/pkg/providers"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeConfigProvider implements the ClusterProvider interface
type KubeConfigProvider struct {
	KubeConfigPath string
	logger         *slog.Logger
}

// New is the exported function required by the plugin system
func New(config map[string]string, logger *slog.Logger) providers.Provider {
	return NewKubeConfigProvider(config, logger)
}

// NewKubeConfigProvider creates a new KubeConfigProvider
func NewKubeConfigProvider(config map[string]string, logger *slog.Logger) *KubeConfigProvider {
	kubeConfigPath, ok := config["kubeconfigPath"]
	if !ok {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			panic(fmt.Sprintf("Failed to get user home directory: %v", err))
		}
		// Default to ~/.kube/config
		kubeConfigPath = filepath.Join(homeDir, ".kube", "config")
	}

	return &KubeConfigProvider{
		KubeConfigPath: kubeConfigPath,
		logger:         logger,
	}
}

// DiscoverClusters discovers clusters from the kubeconfig file
func (p *KubeConfigProvider) DiscoverClusters() ([]providers.ClusterConfig, error) {
	// Load the kubeconfig file
	config, err := clientcmd.LoadFromFile(p.KubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig file: %w", err)
	}

	// Extract cluster and context information
	clusters := make([]providers.ClusterConfig, 0, len(config.Contexts))
	for contextName, context := range config.Contexts {
		clusterName := context.Cluster
		cluster, exists := config.Clusters[clusterName]
		if !exists {
			return nil, fmt.Errorf("context %s references unknown cluster %s", contextName, clusterName)
		}

		clusters = append(clusters, providers.ClusterConfig{
			ID:             contextName, // Use the context name as the cluster ID
			KubeconfigPath: p.KubeConfigPath,
		})

		p.logger.Info("Discovered cluster", "clusterName", clusterName, "server", cluster.Server, "contextName", contextName)
	}

	return clusters, nil
}

func (p *KubeConfigProvider) Authenticate(clusterID string) (*rest.Config, error) {
	// Load the kubeconfig file
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: p.KubeConfigPath}
	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: clusterID,
	}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	// Return the rest.Config
	return clientConfig.ClientConfig()
}
