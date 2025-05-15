package namespaces

import (
	"fmt"
	"log/slog"
	"time"

	"encoding/json"

	resources "github.com/jbetancur/dashboard/internal/pkg/assets"
	messagingtypes "github.com/jbetancur/dashboard/internal/pkg/messaging/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Manager handles namespace-related operations
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

// StartInformer starts the namespace informer
func (nm *Manager) StartInformer() error {
	// Get the namespace informer
	namespaceInformer := nm.informer.Core().V1().Namespaces().Informer()
	if _, err := namespaceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ns := obj.(*v1.Namespace)

			payload := resources.ResourcePayload[v1.Namespace]{
				ClusterID: nm.clusterID,
				Resource:  *ns,
			}

			nsBytes, err := json.Marshal(payload)
			if err != nil {
				nm.logger.Error("failed to serialize namespace", "error", err)
				return
			}

			if err := nm.eventPublisher.Publish("namespace_added", nsBytes); err != nil {
				nm.logger.Error("failed to publish namespace_added event", "error", err)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			ns := newObj.(*v1.Namespace)

			payload := resources.ResourcePayload[v1.Namespace]{
				ClusterID: nm.clusterID,
				Resource:  *ns,
			}
			nsBytes, err := json.Marshal(payload)
			if err != nil {
				nm.logger.Error("failed to serialize namespace", "error", err)
				return
			}

			if err := nm.eventPublisher.Publish("namespace_updated", nsBytes); err != nil {
				nm.logger.Error("failed to publish namespace_updated event", "error", err)
			}
		},
		DeleteFunc: func(obj interface{}) {
			ns := obj.(*v1.Namespace)

			payload := resources.ResourcePayload[v1.Namespace]{
				ClusterID: nm.clusterID,
				Resource:  *ns,
			}

			nsBytes, err := json.Marshal(payload)
			if err != nil {
				nm.logger.Error("failed to serialize namespace", "error", err)
				return
			}

			if err := nm.eventPublisher.Publish("namespace_deleted", nsBytes); err != nil {
				nm.logger.Error("failed to publish namespace_deleted event", "error", err)
			}
		},
	}); err != nil {
		return fmt.Errorf("failed to add event handler to namespace informer: %w", err)
	}

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
