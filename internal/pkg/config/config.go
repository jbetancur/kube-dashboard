package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	assets "github.com/jbetancur/dashboard/internal/pkg/assets"
	"github.com/jbetancur/dashboard/internal/pkg/cluster"
	"github.com/jbetancur/dashboard/internal/pkg/messaging"
	messagingtypes "github.com/jbetancur/dashboard/internal/pkg/messaging/types"
	"github.com/jbetancur/dashboard/internal/pkg/store"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

type ProviderConfig struct {
	Name   string            `yaml:"name"`
	Path   string            `yaml:"path"`
	Config map[string]string `yaml:"config"`
}

type AuthenticatorConfig struct {
	Name   string            `yaml:"name"`
	Path   string            `yaml:"path"`
	Config map[string]string `yaml:"config"`
}

type AppConfig struct {
	Providers      []ProviderConfig      `yaml:"providers"`
	Authenticators []AuthenticatorConfig `yaml:"authenticators"`
}

func LoadConfig(filePath string) (*AppConfig, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close() // Explicitly ignoring the error
	}()

	var config AppConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func Store(ctx context.Context, logger *slog.Logger) (store.Repository, error) {
	store, err := store.NewStore(ctx, "mongodb://localhost:27017", "k8s-starship", logger)
	if err != nil {
		logger.Error("Failed to create MongoDB store", "error", err)
		return nil, err
	}

	return store, nil
}

func configMessageClient(logger *slog.Logger) (messagingtypes.MessageQueue, error) {
	// Initialize the messaging client for bidirectional communication
	messagingConfig := messaging.Config{
		Type:          messaging.GRPCProvider,
		ServerAddress: ":50053", // REST API's server address (for receiving)
		ClientAddress: ":50052", // Agent's server address (for sending)
	}

	messagingClient, err := messaging.NewClient(messagingConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create messaging client: %w", err)
	}

	return messagingClient, nil
}

func StartMessageClients(ctx context.Context, logger *slog.Logger) (messagingtypes.MessageQueue, error) {
	messagingClient, err := configMessageClient(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize messaging client: %w", err)
	}

	// Start the server to listen for agent messages
	err = messagingClient.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start messaging server: %w", err)
	}

	// Connect to the agent's server for potential publishing
	err = messagingClient.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect messaging client: %w", err)
	}

	return messagingClient, nil
}

// SetupSubscriptions configures all event subscriptions
func SetupSubscriptions(
	ctx context.Context,
	messagingClient messagingtypes.MessageQueue,
	store store.Repository,
	clusterManager *cluster.Manager,
	logger *slog.Logger,
) {
	// Subscribe to cluster registration events
	messagingClient.Subscribe("cluster_registered", func(message []byte) error {
		return handleClusterRegistration(ctx, message, clusterManager, store, logger)
	})

	// Subscribe to pod events
	messagingClient.Subscribe("pod_added", func(message []byte) error {
		return handlePodEvent(ctx, message, store, logger)
	})

	// Subscribe to namespace events
	messagingClient.Subscribe("namespace_added", func(message []byte) error {
		return handleNamespaceEvent(ctx, message, store, logger)
	})

	// Log successful subscription setup
	logger.Info("Event subscriptions configured")
}

// handleClusterRegistration processes cluster registration events
func handleClusterRegistration(
	ctx context.Context,
	message []byte,
	clusterManager *cluster.Manager,
	store store.Repository,
	logger *slog.Logger,
) error {
	var payload cluster.ConnectionPayload
	if err := json.Unmarshal(message, &payload); err != nil {
		logger.Error("Failed to unmarshal cluster connection event", "error", err)
		return err
	}

	// Register the cluster
	if err := clusterManager.Register(payload.ClusterName, payload.APIURL); err != nil {
		logger.Error("Failed to register cluster", "error", err)
		return err
	}

	// var payload assets.ResourcePayload[corev1.ClusterInfo]
	// Create a ClusterInfo object to store in the database
	clusterInfo := cluster.ClusterInfo{
		Kind:   "Cluster",
		Name:   payload.ClusterName,
		APIURL: payload.APIURL,
	}

	// Save the cluster to the database
	if err := store.SaveCluster(ctx, &clusterInfo); err != nil {
		logger.Error("Failed to store cluster", "error", err)
		return err
	}

	logger.Info("Registered cluster from event",
		"name", payload.ClusterName,
		"api_url", payload.APIURL)
	return nil
}

// handlePodEvent processes pod events
func handlePodEvent(
	ctx context.Context,
	message []byte,
	store store.Repository,
	logger *slog.Logger,
) error {
	var payload assets.ResourcePayload[corev1.Pod]
	if err := json.Unmarshal(message, &payload); err != nil {
		logger.Error("Failed to unmarshal pod event", "error", err)
		return err
	}

	if err := store.Save(ctx, payload.ClusterID, &payload.Resource); err != nil {
		logger.Error("Failed to store pod", "error", err)
		return err
	}

	logger.Debug("Stored pod from event",
		"name", payload.Resource.Name,
		"namespace", payload.Resource.Namespace,
		"cluster", payload.ClusterID)
	return nil
}

// handleNamespaceEvent processes namespace events
func handleNamespaceEvent(
	ctx context.Context,
	message []byte,
	store store.Repository,
	logger *slog.Logger,
) error {
	var payload assets.ResourcePayload[corev1.Namespace]
	if err := json.Unmarshal(message, &payload); err != nil {
		logger.Error("Failed to unmarshal namespace event", "error", err)
		return err
	}

	if err := store.Save(ctx, payload.ClusterID, &payload.Resource); err != nil {
		logger.Error("Failed to store namespace", "error", err)
		return err
	}

	logger.Info("Stored namespace from event",
		"name", payload.Resource.Name,
		"cluster", payload.ClusterID)
	return nil
}
