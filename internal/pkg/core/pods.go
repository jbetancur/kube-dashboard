package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/gofiber/websocket/v2"
	"github.com/jbetancur/dashboard/internal/pkg/messaging"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// PodManager handles pod-related operations
type PodManager struct {
	client         *kubernetes.Clientset
	informer       informers.SharedInformerFactory
	eventPublisher *messaging.GRPCClient
	stopCh         chan struct{}
}

// NewPodManager creates a new PodManager
func NewPodManager(eventPublisher *messaging.GRPCClient, client *kubernetes.Clientset) *PodManager {
	// Create a shared informer factory
	informer := informers.NewSharedInformerFactory(client, time.Minute*5)

	return &PodManager{
		client:         client,
		informer:       informer,
		eventPublisher: eventPublisher,
		stopCh:         make(chan struct{}),
	}
}

// StartInformer starts the pod informer
func (pm *PodManager) StartInformer() error {
	// Get the pod informer
	podInformer := pm.informer.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			podBytes, err := json.Marshal(pod)
			if err != nil {
				fmt.Printf("failed to serialize namespace: %v\n", err)
				return
			}
			pm.eventPublisher.Publish("pod_added", podBytes)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			pod := newObj.(*v1.Pod)
			podBytes, err := json.Marshal(pod)
			if err != nil {
				fmt.Printf("failed to serialize namespace: %v\n", err)
				return
			}
			pm.eventPublisher.Publish("pod_updated", podBytes)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			podBytes, err := json.Marshal(pod)
			if err != nil {
				fmt.Printf("failed to serialize namespace: %v\n", err)
				return
			}
			pm.eventPublisher.Publish("pod_deleted", podBytes)
		},
	})

	// Start the informer
	go podInformer.Run(pm.stopCh)

	// Wait for the cache to sync
	if !cache.WaitForCacheSync(pm.stopCh, podInformer.HasSynced) {
		return fmt.Errorf("failed to sync pod informer")
	}

	return nil
}

// ListPods retrieves all pods in a specific namespace
func (pm *PodManager) ListPods(ctx context.Context, namespace string) ([]v1.Pod, error) {
	podLister := pm.informer.Core().V1().Pods().Lister().Pods(namespace).List
	podPointers, err := ListResources(ctx, podLister)
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
func (pm *PodManager) GetPod(ctx context.Context, namespace, podName string) (*v1.Pod, error) {
	podGetter := pm.informer.Core().V1().Pods().Lister().Pods(namespace).Get
	return GetResource(ctx, podName, podGetter)
}

// DeletePod deletes a pod by name
func (pm *PodManager) DeletePod(ctx context.Context, namespace, podName string) error {
	return pm.client.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}

// StreamPodLogs streams logs from a pod to a WebSocket connection
func (pm *PodManager) StreamPodLogs(ctx context.Context, namespace, podName, containerName string, conn *websocket.Conn) error {
	// Set up log options
	podLogOpts := &v1.PodLogOptions{
		Follow: true,
	}
	if containerName != "" {
		podLogOpts.Container = containerName
	}

	// Create a request to stream logs
	req := pm.client.CoreV1().Pods(namespace).GetLogs(podName, podLogOpts)
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

// Stop stops the pod manager
func (pm *PodManager) Stop() {
	close(pm.stopCh)
}
