package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jbetancur/dashboard/internal/pkg/grpc"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ConnectionPayload represents the payload sent to the REST API
type ConnectionPayload struct {
	ClusterName string `json:"clusterName"`
	APIURL      string `json:"apiURL"`
}

// Connection represents a connection to a Kubernetes cluster
type Connection struct {
	ID       string
	Client   *kubernetes.Clientset
	Config   *rest.Config // Make sure this field exists
	Informer informers.SharedInformerFactory
	StopCh   chan struct{}
	AuthDone bool
	Running  bool // Tracks whether informers are running
}

// NewConnection creates a new cluster connection
func NewConnection(id string, client *kubernetes.Clientset, config *rest.Config) *Connection {
	return &Connection{
		ID:       id,
		Client:   client,
		Config:   config, // Store the config
		StopCh:   make(chan struct{}),
		AuthDone: true,
		Running:  false,
	}
}

// InitializeInformers creates the informer factory for this cluster
func (c *Connection) InitializeInformers() {
	if c.Informer == nil && c.Client != nil {
		c.Informer = informers.NewSharedInformerFactory(c.Client, 5*time.Minute)
	}
}

// Stop stops this cluster's informers
func (c *Connection) Stop() {
	select {
	case <-c.StopCh:
		// Already closed
	default:
		close(c.StopCh)
	}
	c.Running = false
}

// PublishConnection sends the cluster connection details via the message queue
func PublishConnection(messageQueue *grpc.GRPCClient, clusterName, apiServerURL string, logger *slog.Logger) error {
	payload := ConnectionPayload{
		ClusterName: clusterName,
		APIURL:      apiServerURL,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal cluster connection payload: %w", err)
	}

	// Retry logic
	for i := 0; i < 5; i++ {
		err = messageQueue.Publish("cluster_registered", data)
		if err == nil {
			return nil
		}

		logger.Warn("Failed to publish cluster connection, retrying...", "attempt", i+1, "error", err)
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("failed to publish cluster connection after retries: %w", err)
}

// IsConnected returns whether the cluster is connected and authenticated
func (c *Connection) IsConnected() bool {
	return c.Client != nil && c.AuthDone
}

// StartInformers begins all informers for this cluster
func (c *Connection) StartInformers() {
	if c.Running || c.Informer == nil {
		return
	}

	c.Informer.Start(c.StopCh)
	c.Running = true
}

// GetHealthStatus provides health check status for the cluster
func (c *Connection) GetHealthStatus() (bool, error) {
	// Basic check: try listing namespaces
	if c.Client == nil {
		return false, fmt.Errorf("client not initialized")
	}

	_, err := c.Client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{Limit: 1})
	if err != nil {
		return false, err
	}

	return true, nil
}
