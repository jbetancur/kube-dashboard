package core

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MultiClusterPodProvider implements PodProvider for multiple clusters
type MultiClusterPodProvider struct {
	clusterManager *ClusterManager
}

// NewMultiClusterPodProvider creates a new provider
func NewMultiClusterPodProvider(clusterManager *ClusterManager) *MultiClusterPodProvider {
	return &MultiClusterPodProvider{
		clusterManager: clusterManager,
	}
}

// ListPods lists pods in a specific namespace from a specific cluster
func (p *MultiClusterPodProvider) ListPods(ctx context.Context, clusterID, namespace string) ([]v1.Pod, error) {
	// Get the cluster connection
	cluster, err := p.clusterManager.GetCluster(clusterID)
	if err != nil {
		return nil, fmt.Errorf("cluster not found: %w", err)
	}

	// Use the Kubernetes API directly (no informers)
	podList, err := cluster.Client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	return podList.Items, nil
}

// GetPod gets a specific pod from a specific cluster and namespace
func (p *MultiClusterPodProvider) GetPod(ctx context.Context, clusterID, namespace, podName string) (*v1.Pod, error) {
	// Get the cluster connection
	cluster, err := p.clusterManager.GetCluster(clusterID)
	if err != nil {
		return nil, fmt.Errorf("cluster not found: %w", err)
	}

	// Use the Kubernetes API directly (no informers)
	pod, err := cluster.Client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}

	return pod, nil
}
