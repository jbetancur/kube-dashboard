package pods

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jbetancur/dashboard/internal/pkg/messaging"
	"github.com/jbetancur/dashboard/internal/pkg/resources"

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
	eventPublisher *messaging.GRPCClient
	logger         *slog.Logger
	stopCh         chan struct{}
}

// NewManager creates a new Manager
func NewManager(
	clusterID string,
	eventPublisher *messaging.GRPCClient,
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
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
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
			pm.eventPublisher.Publish("pod_added", podBytes)
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
			pm.eventPublisher.Publish("pod_updated", podBytes)
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

// Stop stops the pod manager
func (pm *Manager) Stop() {
	close(pm.stopCh)
}
