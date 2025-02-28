package providers

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jbetancur/dashboard/internal/pkg/core"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeConfigProvider implements the ClusterProvider interface for the TUI
type KubeConfigProvider struct {
	KubeConfigPath string
	logger         *slog.Logger
}

// NewAPIKubeConfigProvider creates a new KubeConfigProvider
func NewKubeConfigProvider(logger *slog.Logger) *KubeConfigProvider {
	// Default to ~/.kube/config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("Failed to get user home directory: %v", err))
	}

	return &KubeConfigProvider{
		KubeConfigPath: filepath.Join(homeDir, ".kube", "config"),
		logger:         logger,
	}
}

func (p *KubeConfigProvider) DiscoverClusters() ([]core.ClusterConfig, error) {
	// Load the kubeconfig file
	config, err := clientcmd.LoadFromFile(p.KubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig file: %w", err)
	}

	// Extract cluster and context information
	clusters := make([]core.ClusterConfig, 0, len(config.Contexts))
	for contextName, context := range config.Contexts {
		clusterName := context.Cluster
		cluster, exists := config.Clusters[clusterName]
		if !exists {
			return nil, fmt.Errorf("context %s references unknown cluster %s", contextName, clusterName)
		}

		clusters = append(clusters, core.ClusterConfig{
			ID:             contextName, // Use the context name as the cluster ID
			KubeconfigPath: p.KubeConfigPath,
		})

		p.logger.Info("Discovered cluster", "clusterName", clusterName, "server", cluster.Server, "contextName", contextName)
	}

	return clusters, nil
}
