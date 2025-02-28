package core

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

// NamespaceManager handles namespace-related operations
type NamespaceManager struct {
	clusterGetter ClusterGetter
}

// NewNamespaceManager creates a new NamespaceManager
func NewNamespaceManager(cg ClusterGetter, clusters []ClusterConfig) (*NamespaceManager, error) {
	nm := &NamespaceManager{
		clusterGetter: cg,
	}

	// Use the generic StartInformers function
	err := StartInformers(clusters, nm.StartNamespaceInformer)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize NamespaceManager: %w", err)
	}

	return nm, nil
}

// StartNamespaceInformer starts the namespace informer for a specific cluster
func (nm *NamespaceManager) StartNamespaceInformer(clusterID string) error {
	// Get the cluster from the ClusterManager
	cluster, err := nm.clusterGetter.GetCluster(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	// Get the namespace informer
	namespaceInformer := cluster.Informer.Core().V1().Namespaces().Informer()
	namespaceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// ns := obj.(*v1.Namespace)
			// fmt.Printf("Namespace added: %s/%s\n", clusterID, ns.Name)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// ns := newObj.(*v1.Namespace)
			// fmt.Printf("Namespace updated: %s/%s\n", clusterID, ns.Name)
		},
		DeleteFunc: func(obj interface{}) {
			// ns := obj.(*v1.Namespace)
			// fmt.Printf("Namespace deleted: %s/%s\n", clusterID, ns.Name)
		},
	})

	// Start the informer
	go namespaceInformer.Run(cluster.StopCh)

	// Wait for the cache to sync
	if !cache.WaitForCacheSync(cluster.StopCh, namespaceInformer.HasSynced) {
		return fmt.Errorf("failed to sync namespace informer for cluster %s", clusterID)
	}

	return nil
}

// ListNamespaces retrieves all namespaces for a given cluster
// ListNamespaces retrieves all namespaces for a given cluster
func (nm *NamespaceManager) ListNamespaces(ctx context.Context, clusterID string) ([]v1.Namespace, error) {
	// Get the cluster from the ClusterManager
	cluster, err := nm.clusterGetter.GetCluster(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	namespaceLister := cluster.Informer.Core().V1().Namespaces().Lister().List

	namespacesPtr, err := ListResources(ctx, clusterID, namespaceLister)
	if err != nil {
		return nil, err
	}

	namespaces := make([]v1.Namespace, len(namespacesPtr))
	for i, nsPtr := range namespacesPtr {
		namespaces[i] = *nsPtr
	}

	return namespaces, nil
}

// GetNamespace retrieves a specific namespace by name for a given cluster
func (nm *NamespaceManager) GetNamespace(ctx context.Context, clusterID, namespaceName string) (*v1.Namespace, error) {
	// Get the cluster from the ClusterManager
	cluster, err := nm.clusterGetter.GetCluster(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	namespaceGetter := cluster.Informer.Core().V1().Namespaces().Lister().Get

	return GetResource(ctx, clusterID, namespaceName, namespaceGetter)
}
