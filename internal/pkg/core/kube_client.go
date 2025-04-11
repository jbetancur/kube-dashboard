package core

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

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

// NewKubeClient creates a new Kubernetes client configuration
func NewKubeClient(logger *slog.Logger) ([]*ClusterConfig, error) {
	var kubeconfig string
	var err error
	var configs []*ClusterConfig

	// Check if running inside the cluster
	inClusterConfig, err := rest.InClusterConfig()
	if err == nil {
		// Running inside the cluster
		clientset, err := kubernetes.NewForConfig(inClusterConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create in-cluster client: %w", err)
		}

		// Return a single configuration for the in-cluster setup
		configs = append(configs, &ClusterConfig{
			Client:     clientset,
			Config:     inClusterConfig,
			Kubeconfig: "",
			Cluster:    "in-cluster",
		})
		return configs, nil
	}

	// Running outside the cluster, load kubeconfig
	logger.Warn("Running outside the cluster, loading kubeconfig")

	// Load kubeconfig from the default location or KUBECONFIG environment variable
	kubeconfig = os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home := os.Getenv("HOME")
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	// Check if the kubeconfig file exists
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		return nil, fmt.Errorf("kubeconfig file does not exist at path: %s", kubeconfig)
	}

	// Load the kubeconfig
	rawConfig, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig file: %w", err)
	}

	// Iterate through all contexts in the kubeconfig
	for contextName := range rawConfig.Contexts {
		logger.Info("Processing context", "name", contextName)

		// Build the rest.Config for this context
		config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
			&clientcmd.ConfigOverrides{CurrentContext: contextName},
		).ClientConfig()
		if err != nil {
			logger.Warn("Failed to load context", "context", contextName, "error", err)
			continue
		}

		// Create a Kubernetes client for this context
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			logger.Warn("Failed to create client for context", "context", contextName, "error", err)
			continue
		}

		// Append the configuration to the slice
		configs = append(configs, &ClusterConfig{
			Client:     clientset,
			Config:     config,
			Kubeconfig: kubeconfig,
			Cluster:    contextName,
		})
	}

	if len(configs) == 0 {
		return nil, fmt.Errorf("no valid clusters found in kubeconfig")
	}

	return configs, nil
}
