package core

// ClusterConfig represents the configuration for a cluster
type ClusterConfig struct {
	ID             string
	KubeconfigPath string
}

type ClusterProvider interface {
	DiscoverClusters() ([]ClusterConfig, error)
}
