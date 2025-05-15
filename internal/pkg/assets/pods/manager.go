package pods

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	resources "github.com/jbetancur/dashboard/internal/pkg/assets"
	messagingtypes "github.com/jbetancur/dashboard/internal/pkg/messaging/types"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Manager handles pod-related operations
type Manager struct {
	clusterID      string
	client         *kubernetes.Clientset
	informer       informers.SharedInformerFactory
	eventPublisher messagingtypes.Publisher
	logger         *slog.Logger
	stopCh         chan struct{}
}

// NewManager creates a new Manager
func NewManager(
	clusterID string,
	eventPublisher messagingtypes.Publisher,
	client *kubernetes.Clientset,
	logger *slog.Logger,
) *Manager {
	// Create a shared informer factory
	informer := informers.NewSharedInformerFactory(client, time.Minute*5)

	return &Manager{
		clusterID:      clusterID,
		client:         client,
		informer:       informer,
		eventPublisher: eventPublisher,
		logger:         logger,
		stopCh:         make(chan struct{}),
	}
}

// StartInformer starts the pod informer
func (pm *Manager) StartInformer() error {
	// Get the pod informer
	podInformer := pm.informer.Core().V1().Pods().Informer()
	if _, err := podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			payload := resources.ResourcePayload[v1.Pod]{
				ClusterID: pm.clusterID,
				Resource:  *pod,
			}

			podBytes, err := json.Marshal(payload)
			if err != nil {
				pm.logger.Error("failed to serialize pod", "error", err)
				return
			}
			if err := pm.eventPublisher.Publish("pod_added", podBytes); err != nil {
				pm.logger.Error("failed to publish pod addition", "error", err)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			pod := newObj.(*v1.Pod)
			payload := resources.ResourcePayload[v1.Pod]{
				ClusterID: pm.clusterID,
				Resource:  *pod,
			}
			podBytes, err := json.Marshal(payload)
			if err != nil {
				pm.logger.Error("failed to serialize pod", "error", err)
				return
			}
			if err := pm.eventPublisher.Publish("pod_updated", podBytes); err != nil {
				pm.logger.Error("failed to publish pod update", "error", err)
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			payload := resources.ResourcePayload[v1.Pod]{
				ClusterID: pm.clusterID,
				Resource:  *pod,
			}
			podBytes, err := json.Marshal(payload)
			if err != nil {
				pm.logger.Error("failed to serialize pod", "error", err)
				return
			}
			if err := pm.eventPublisher.Publish("pod_deleted", podBytes); err != nil {
				pm.logger.Error("failed to publish pod deletion", "error", err)
			}
		},
	}); err != nil {
		return fmt.Errorf("failed to add pod event handler: %w", err)
	}

	// Start the informer
	go podInformer.Run(pm.stopCh)

	// Wait for the cache to sync
	if !cache.WaitForCacheSync(pm.stopCh, podInformer.HasSynced) {
		return fmt.Errorf("failed to sync pod informer")
	}

	return nil
}

// Stop stops the pod manager
func (pm *Manager) Stop() {
	close(pm.stopCh)
}
