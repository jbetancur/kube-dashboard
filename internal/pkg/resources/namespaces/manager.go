package namespaces

import (
	"fmt"
	"time"

	"encoding/json"

	"github.com/jbetancur/dashboard/internal/pkg/messaging"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Manager handles namespace-related operations
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

// StartInformer starts the namespace informer
func (nm *Manager) StartInformer() error {
	// Get the namespace informer
	namespaceInformer := nm.informer.Core().V1().Namespaces().Informer()
	namespaceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ns := obj.(*v1.Namespace)
			nsBytes, err := json.Marshal(ns)
			if err != nil {
				fmt.Printf("failed to serialize namespace: %v\n", err)
				return
			}
			nm.eventPublisher.Publish("namespace_added", nsBytes)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			ns := newObj.(*v1.Namespace)
			nsBytes, err := json.Marshal(ns)
			if err != nil {
				fmt.Printf("failed to serialize namespace: %v\n", err)
				return
			}
			nm.eventPublisher.Publish("namespace_updated", nsBytes)
		},
		DeleteFunc: func(obj interface{}) {
			ns := obj.(*v1.Namespace)
			nsBytes, err := json.Marshal(ns)
			if err != nil {
				fmt.Printf("failed to serialize namespace: %v\n", err)
				return
			}
			nm.eventPublisher.Publish("namespace_deleted", nsBytes)
		},
	})

	// Start the informer
	go namespaceInformer.Run(nm.stopCh)

	// Wait for the cache to sync
	if !cache.WaitForCacheSync(nm.stopCh, namespaceInformer.HasSynced) {
		return fmt.Errorf("failed to sync namespace informer")
	}

	return nil
}

// Stop stops the namespace manager
func (nm *Manager) Stop() {
	close(nm.stopCh)
}
