package namespaces

import (
	"context"
	"fmt"
	"time"

	"github.com/jbetancur/dashboard/internal/pkg/cluster"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	informersv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// NamespaceProvider implements NamespaceProvider for multiple clusters
type NamespaceProvider struct {
	clusterManager *cluster.Manager
}

// NewNamespaceProvider creates a new provider
func NewNamespaceProvider(clusterManager *cluster.Manager) *NamespaceProvider {
	return &NamespaceProvider{
		clusterManager: clusterManager,
	}
}

// ListNamespaces lists namespaces from a specific cluster
func (p *NamespaceProvider) ListNamespaces(ctx context.Context, clusterID string) ([]v1.Namespace, error) {
	// Get the cluster connection
	conn, err := p.clusterManager.GetCluster(clusterID)
	if err != nil {
		return nil, fmt.Errorf("cluster not found: %w", err)
	}

	// Check if the informer factory is initialized
	if conn.Informer == nil {
		return nil, fmt.Errorf("informer factory not initialized for cluster %s", clusterID)
	}

	// Get the namespace informer - ensure it's started
	namespaceInformer := getNamespaceInformer(conn.Informer, "")
	if !cache.WaitForCacheSync(ctx.Done(), namespaceInformer.Informer().HasSynced) {
		return nil, fmt.Errorf("timed out waiting for namespace cache to sync")
	}

	// Use the lister to get namespaces from cache
	nsList, err := namespaceInformer.Lister().List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces from cache: %w", err)
	}

	// Convert from *v1.Namespace to v1.Namespace
	namespaces := make([]v1.Namespace, 0, len(nsList))
	for _, ns := range nsList {
		namespaces = append(namespaces, *ns.DeepCopy())
	}

	return namespaces, nil
}

// GetNamespace gets a specific namespace from a specific cluster
func (p *NamespaceProvider) GetNamespace(ctx context.Context, clusterID, namespaceName string) (*v1.Namespace, error) {
	// Get the cluster connection
	conn, err := p.clusterManager.GetCluster(clusterID)
	if err != nil {
		return nil, fmt.Errorf("cluster not found: %w", err)
	}

	// Check if the informer factory is initialized
	if conn.Informer == nil {
		return nil, fmt.Errorf("informer factory not initialized for cluster %s", clusterID)
	}

	// Get the namespace informer - ensure it's started
	namespaceInformer := getNamespaceInformer(conn.Informer, "")
	if !cache.WaitForCacheSync(ctx.Done(), namespaceInformer.Informer().HasSynced) {
		return nil, fmt.Errorf("timed out waiting for namespace cache to sync")
	}

	// Get from cache
	ns, err := namespaceInformer.Lister().Get(namespaceName)
	if err != nil {
		// If not found in cache or other error, try direct API call as fallback
		return conn.Client.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
	}

	// Return a deep copy to avoid cache mutation
	return ns.DeepCopy(), nil
}

// Helper function to get or create the namespace informer
func getNamespaceInformer(factory informers.SharedInformerFactory, namespace string) informersv1.NamespaceInformer {
	if namespace != "" {
		return factory.InformerFor(
			&v1.Namespace{},
			func(client kubernetes.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
				return informersv1.NewNamespaceInformer(
					client,
					resyncPeriod,
					cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
				)
			},
		).(informersv1.NamespaceInformer)
	}

	// Use factory's standard namespace informer for all namespaces
	return factory.Core().V1().Namespaces()
}

// EnsureInformersStarted makes sure the informers are started for the given cluster
func (p *NamespaceProvider) EnsureInformersStarted(clusterID string) error {
	conn, err := p.clusterManager.GetCluster(clusterID)
	if err != nil {
		return fmt.Errorf("cluster not found: %w", err)
	}

	if conn.Informer == nil {
		conn.InitializeInformers()
	}

	// Check if informers are running
	if !conn.Running {
		conn.StartInformers()

		// Wait a short time for initial sync
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Wait for namespace informer to sync
		namespaceInformer := conn.Informer.Core().V1().Namespaces().Informer()
		if !cache.WaitForCacheSync(ctx.Done(), namespaceInformer.HasSynced) {
			return fmt.Errorf("timed out waiting for namespace cache to sync")
		}
	}

	return nil
}
