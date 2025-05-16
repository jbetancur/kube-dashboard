package store

import (
	"context"

	"github.com/jbetancur/dashboard/internal/pkg/cluster"
	"k8s.io/apimachinery/pkg/runtime"
)

// Repository defines the interface for storage operations
type Repository interface {
	// Save stores a Kubernetes resource
	Save(ctx context.Context, clusterID string, obj runtime.Object) error

	// SaveCluster stores cluster information
	SaveCluster(ctx context.Context, clusterInfo *cluster.ClusterInfo) error

	// Get retrieves a Kubernetes resource
	Get(ctx context.Context, clusterID, namespace, kind, name string, result interface{}) error

	// GetCluster retrieves cluster information
	GetCluster(ctx context.Context, name string, result *cluster.ClusterInfo) error

	// List returns resources matching criteria
	List(ctx context.Context, clusterID, namespace, kind string, results interface{}) error

	//ListCluster returns all clusters
	ListClusters(ctx context.Context, results *[]cluster.ClusterInfo) error

	// Delete removes a resource
	Delete(ctx context.Context, clusterID, namespace, kind, name string) error

	// DeleteByFilter removes resources matching a filter
	DeleteByFilter(ctx context.Context, filter map[string]interface{}) error

	// Close shuts down the repository
	Close(ctx context.Context) error
}
