package core

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

// NamespaceManager handles namespace-related operations
type NamespaceManager struct {
	client         *kubernetes.Clientset
	informer       informers.SharedInformerFactory
	eventPublisher *messaging.GRPCClient
	stopCh         chan struct{}
}

// NewNamespaceManager creates a new NamespaceManager
func NewNamespaceManager(eventPublisher *messaging.GRPCClient, client *kubernetes.Clientset) *NamespaceManager {
	// Create a shared informer factory
	informer := informers.NewSharedInformerFactory(client, time.Minute*5)

	return &NamespaceManager{
		client:         client,
		informer:       informer,
		eventPublisher: eventPublisher,
		stopCh:         make(chan struct{}),
	}
}

// StartInformer starts the namespace informer
func (nm *NamespaceManager) StartInformer() error {
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
func (nm *NamespaceManager) Stop() {
	close(nm.stopCh)
}
