package providers

import "k8s.io/client-go/rest"

// ClusterConfig represents the configuration for a cluster
type ClusterConfig struct {
	ID             string
	KubeconfigPath string
}

type Provider interface {
	DiscoverClusters() ([]ClusterConfig, error)
	Authenticate(clusterID string) (*rest.Config, error)
}
