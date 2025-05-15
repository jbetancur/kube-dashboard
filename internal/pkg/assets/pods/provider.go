package pods

import (
	"context"
	"fmt"
	"io"

	"github.com/jbetancur/dashboard/internal/pkg/cluster"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MultiClusterPodProvider implements PodProvider for multiple clusters
type MultiClusterPodProvider struct {
	clusterManager *cluster.Manager
}

// NewMultiClusterPodProvider creates a new provider
func NewMultiClusterPodProvider(clusterManager *cluster.Manager) *MultiClusterPodProvider {
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

// Add this method to MultiClusterPodProvider
func (p *MultiClusterPodProvider) GetPodLogs(ctx context.Context, clusterID, namespace, podName, containerName string, tailLines int64) (io.ReadCloser, error) {
	// Get the cluster connection
	conn, err := p.clusterManager.GetCluster(clusterID)
	if err != nil {
		return nil, fmt.Errorf("cluster not found: %w", err)
	}

	// Prepare log options
	options := &v1.PodLogOptions{
		Container: containerName,
		Follow:    true,
	}

	if tailLines > 0 {
		options.TailLines = &tailLines
	}

	// Get the logs stream
	return conn.Client.CoreV1().Pods(namespace).GetLogs(podName, options).Stream(ctx)
}
