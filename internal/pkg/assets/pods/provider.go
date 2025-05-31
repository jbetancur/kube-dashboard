package pods

import (
	"context"
	"fmt"
	"io"
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

// PodProvider implements PodProvider for multiple clusters
type PodProvider struct {
	clusterManager *cluster.Manager
}

// NewPodProvider creates a new provider
func NewPodProvider(clusterManager *cluster.Manager) *PodProvider {
	return &PodProvider{
		clusterManager: clusterManager,
	}
}

// ListPods lists pods in a specific namespace from a specific cluster
func (p *PodProvider) ListPods(ctx context.Context, clusterID, namespace string) ([]v1.Pod, error) {
	// Get the cluster connection
	conn, err := p.clusterManager.GetCluster(clusterID)
	if err != nil {
		return nil, fmt.Errorf("cluster not found: %w", err)
	}

	// Check if the informer factory is initialized
	if conn.Informer == nil {
		return nil, fmt.Errorf("informer factory not initialized for cluster %s", clusterID)
	}

	// Get the pod informer - ensure it's started
	podInformer := getPodInformer(conn.Informer, namespace)
	if !cache.WaitForCacheSync(ctx.Done(), podInformer.Informer().HasSynced) {
		return nil, fmt.Errorf("timed out waiting for pod cache to sync")
	}

	// Use the lister to get pods from cache
	var pods []v1.Pod
	if namespace != "" {
		// List from specific namespace
		nsLister := podInformer.Lister().Pods(namespace)
		podList, err := nsLister.List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("failed to list pods from cache: %w", err)
		}

		// Convert from *v1.Pod to v1.Pod
		pods = make([]v1.Pod, 0, len(podList))
		for _, pod := range podList {
			pods = append(pods, *pod.DeepCopy())
		}
	} else {
		// List across all namespaces
		podList, err := podInformer.Lister().List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("failed to list pods from cache: %w", err)
		}

		// Convert from *v1.Pod to v1.Pod
		pods = make([]v1.Pod, 0, len(podList))
		for _, pod := range podList {
			pods = append(pods, *pod.DeepCopy())
		}
	}

	return pods, nil
}

// GetPod gets a specific pod from a specific cluster and namespace
func (p *PodProvider) GetPod(ctx context.Context, clusterID, namespace, podName string) (*v1.Pod, error) {
	// Get the cluster connection
	conn, err := p.clusterManager.GetCluster(clusterID)
	if err != nil {
		return nil, fmt.Errorf("cluster not found: %w", err)
	}

	// Check if the informer factory is initialized
	if conn.Informer == nil {
		return nil, fmt.Errorf("informer factory not initialized for cluster %s", clusterID)
	}

	// Get the pod informer - ensure it's started
	podInformer := getPodInformer(conn.Informer, namespace)
	if !cache.WaitForCacheSync(ctx.Done(), podInformer.Informer().HasSynced) {
		return nil, fmt.Errorf("timed out waiting for pod cache to sync")
	}

	// Get from cache
	nsLister := podInformer.Lister().Pods(namespace)
	pod, err := nsLister.Get(podName)
	if err != nil {
		// If not found in cache or other error, try direct API call as fallback
		return conn.Client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	}

	// Return a deep copy to avoid cache mutation
	return pod.DeepCopy(), nil
}

// GetPodLogs fetches pod logs (we still use direct API call for logs)
func (p *PodProvider) GetPodLogs(ctx context.Context, clusterID, namespace, podName, containerName string, tailLines int64) (io.ReadCloser, error) {
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

	// Get the logs stream (must use direct API call)
	return conn.Client.CoreV1().Pods(namespace).GetLogs(podName, options).Stream(ctx)
}

// Helper function to get or create the pod informer
func getPodInformer(factory informers.SharedInformerFactory, namespace string) informersv1.PodInformer {
	if namespace != "" {
		return factory.InformerFor(
			&v1.Pod{},
			func(client kubernetes.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
				return informersv1.NewPodInformer(
					client,
					namespace,
					resyncPeriod,
					cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
				)
			},
		).(informersv1.PodInformer)
	}

	// Use factory's standard pod informer for all namespaces
	return factory.Core().V1().Pods()
}

// EnsureInformersStarted makes sure the informers are started for the given cluster
func (p *PodProvider) EnsureInformersStarted(clusterID string) error {
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

		// Wait for pod informer to sync
		podInformer := conn.Informer.Core().V1().Pods().Informer()
		if !cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced) {
			return fmt.Errorf("timed out waiting for pod cache to sync")
		}
	}

	return nil
}
