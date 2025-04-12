package pods

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jbetancur/dashboard/internal/pkg/messaging"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Manager handles pod-related operations
type Manager struct {
	client         *kubernetes.Clientset
	informer       informers.SharedInformerFactory
	eventPublisher *messaging.GRPCClient
	stopCh         chan struct{}
}

// NewManager creates a new Manager
func NewManager(eventPublisher *messaging.GRPCClient, client *kubernetes.Clientset) *Manager {
	// Create a shared informer factory
	informer := informers.NewSharedInformerFactory(client, time.Minute*5)

	return &Manager{
		client:         client,
		informer:       informer,
		eventPublisher: eventPublisher,
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

// Stop stops the pod manager
func (pm *Manager) Stop() {
	close(pm.stopCh)
}
