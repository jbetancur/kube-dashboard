package configmaps

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
	// Get the config map informer
	configMapInformer := pm.informer.Core().V1().ConfigMaps().Informer()
	if _, err := configMapInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			configMap := obj.(*v1.ConfigMap)
			payload := resources.ResourcePayload[v1.ConfigMap]{
				ClusterID: pm.clusterID,
				Resource:  *configMap,
			}

			configMapBytes, err := json.Marshal(payload)
			if err != nil {
				pm.logger.Error("failed to serialize config map", "error", err)
				return
			}
			if err := pm.eventPublisher.Publish("config_map_added", configMapBytes); err != nil {
				pm.logger.Error("failed to publish config map addition", "error", err)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			configMap := newObj.(*v1.ConfigMap)
			payload := resources.ResourcePayload[v1.ConfigMap]{
				ClusterID: pm.clusterID,
				Resource:  *configMap,
			}
			configMapBytes, err := json.Marshal(payload)
			if err != nil {
				pm.logger.Error("failed to serialize config map", "error", err)
				return
			}
			if err := pm.eventPublisher.Publish("config_map_updated", configMapBytes); err != nil {
				pm.logger.Error("failed to publish config map update", "error", err)
			}
		},
		DeleteFunc: func(obj interface{}) {
			configMap := obj.(*v1.ConfigMap)
			payload := resources.ResourcePayload[v1.ConfigMap]{
				ClusterID: pm.clusterID,
				Resource:  *configMap,
			}
			configMapBytes, err := json.Marshal(payload)
			if err != nil {
				pm.logger.Error("failed to serialize config map", "error", err)
				return
			}
			if err := pm.eventPublisher.Publish("config_map_deleted", configMapBytes); err != nil {
				pm.logger.Error("failed to publish config map deletion", "error", err)
			}
		},
	}); err != nil {
		return fmt.Errorf("failed to add config map event handler: %w", err)
	}

	// Start the informer
	go configMapInformer.Run(pm.stopCh)

	// Wait for the cache to sync
	if !cache.WaitForCacheSync(pm.stopCh, configMapInformer.HasSynced) {
		return fmt.Errorf("failed to sync config map informer")
	}

	return nil
}

// Stop stops the config map manager
func (pm *Manager) Stop() {
	close(pm.stopCh)
}
