package namespaces

import (
	"fmt"
	"log/slog"
	"time"

	"encoding/json"

	"github.com/jbetancur/dashboard/internal/pkg/grpc"
	"github.com/jbetancur/dashboard/internal/pkg/resources"
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
	eventPublisher *grpc.GRPCClient
	logger         *slog.Logger
	stopCh         chan struct{}
}

// NewManager creates a new Manager
func NewManager(
	clusterID string,
	eventPublisher *grpc.GRPCClient,
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
	namespaceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
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

			nm.eventPublisher.Publish("namespace_added", nsBytes)
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

			nm.eventPublisher.Publish("namespace_updated", nsBytes)
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

			nm.eventPublisher.Publish("namespace_updated", nsBytes)
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
