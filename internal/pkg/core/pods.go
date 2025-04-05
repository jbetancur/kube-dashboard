package core

import (
	"context"
	"fmt"
	"io"

	"github.com/gofiber/websocket/v2"
	"github.com/jbetancur/dashboard/internal/pkg/providers"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

// PodManager handles pod-related operations
type PodManager struct {
	clusterGetter  ClusterGetter
	eventPublisher *EventPublisher
}

// NewPodManager creates a new PodManager
func NewPodManager(eventPublisher *EventPublisher, cg ClusterGetter, clusters []providers.ClusterConfig) (*PodManager, error) {
	pm := &PodManager{
		clusterGetter:  cg,
		eventPublisher: eventPublisher,
	}

	// Use the generic StartInformers function
	err := StartInformers(clusters, pm.StartPodInformer)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize PodManager: %w", err)
	}

	return pm, nil
}

// StartPodInformer starts the pod informer for a specific cluster
func (pm *PodManager) StartPodInformer(clusterID string) error {
	cluster, err := pm.clusterGetter.GetClusterConnection(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	podInformer := cluster.Informer.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			pm.eventPublisher.PublishEvent("pod_added", "pod_events", clusterID, pod)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			pod := newObj.(*v1.Pod)
			pm.eventPublisher.PublishEvent("pod_updated", "pod_events", clusterID, pod)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			pm.eventPublisher.PublishEvent("pod_deleted", "pod_events", clusterID, pod)
		},
	})

	// Start the informer
	go cluster.Informer.Start(cluster.StopCh)

	// Wait for the cache to sync
	if !cache.WaitForCacheSync(cluster.StopCh, podInformer.HasSynced) {
		return fmt.Errorf("failed to sync pod informer for cluster %s", clusterID)
	}

	return nil
}

// ListPods retrieves all pods in a specific namespace for a given cluster
func (pm *PodManager) ListPods(ctx context.Context, clusterID, namespace string) ([]v1.Pod, error) {
	// Get the cluster from the ClusterManager
	cluster, err := pm.clusterGetter.GetClusterConnection(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	podLister := cluster.Informer.Core().V1().Pods().Lister().Pods(namespace).List
	podPointers, err := ListResources(ctx, clusterID, podLister)
	if err != nil {
		return nil, err
	}

	// Convert []*v1.Pod to []v1.Pod
	pods := make([]v1.Pod, len(podPointers))
	for i, podPointer := range podPointers {
		pods[i] = *podPointer
	}

	return pods, nil
}

// GetPod retrieves a specific pod by name
func (pm *PodManager) GetPod(ctx context.Context, clusterID, namespace, podName string) (*v1.Pod, error) {
	cluster, err := pm.clusterGetter.GetClusterConnection(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	podGetter := cluster.Informer.Core().V1().Pods().Lister().Pods(namespace).Get
	return GetResource(ctx, clusterID, podName, podGetter)
}

// DeletePod deletes a pod by name
func (pm *PodManager) DeletePod(ctx context.Context, clusterID, namespace, podName string) error {
	cluster, err := pm.clusterGetter.GetClusterConnection(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	return cluster.Client.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}

// StreamPodLogs streams logs from a pod to a WebSocket connection
func (pm *PodManager) StreamPodLogs(ctx context.Context, clusterID, namespace, podName, containerName string, conn *websocket.Conn) error {
	cluster, err := pm.clusterGetter.GetClusterConnection(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	// Set up log options
	podLogOpts := &v1.PodLogOptions{
		Follow: true,
	}
	if containerName != "" {
		podLogOpts.Container = containerName
	}

	// Create a request to stream logs
	req := cluster.Client.CoreV1().Pods(namespace).GetLogs(podName, podLogOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to open log stream: %w", err)
	}

	defer stream.Close()

	// Stream logs to the WebSocket connection
	buf := make([]byte, 1024)
	for {
		n, err := stream.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading log stream: %w", err)
		}

		// Write the logs to the WebSocket connection
		if err := conn.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
			return fmt.Errorf("error writing to WebSocket: %w", err)
		}
	}

	return nil
}
