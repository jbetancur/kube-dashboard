package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jbetancur/dashboard/internal/pkg/client"
	"github.com/jbetancur/dashboard/internal/pkg/cluster"
	"github.com/jbetancur/dashboard/internal/pkg/messaging"
	messagetypes "github.com/jbetancur/dashboard/internal/pkg/messaging/types"
	"github.com/jbetancur/dashboard/internal/pkg/resources/namespaces"
	"github.com/jbetancur/dashboard/internal/pkg/resources/pods"
)

type ClusterManagers struct {
	Cluster          string
	NamespaceManager *namespaces.Manager
	PodManager       *pods.Manager
	// Add other managers as needed
}

func main() {
	// Initialize logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	logger.Info("Starting cluster agent")

	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("Received shutdown signal", "signal", sig)
		cancel()
	}()

	// Initialize the messaging client
	messagingConfig := messaging.Config{
		Type:          messaging.GRPCProvider,
		ServerAddress: ":50052", // Agent's server address (for receiving)
		ClientAddress: ":50053", // REST API's server address (for sending)
	}

	messagingClient, err := messaging.NewClient(messagingConfig, logger)
	if err != nil {
		logger.Error("Failed to create messaging client", "error", err)
		return
	}

	// Start listening for potential API messages
	err = messagingClient.Start(ctx)
	if err != nil {
		logger.Error("Failed to start messaging server", "error", err)
		return
	}

	// Connect client for publishing
	err = messagingClient.Connect(ctx)
	if err != nil {
		logger.Error("Failed to connect messaging client", "error", err)
		return
	}
	defer func() {
		if err := messagingClient.Stop(); err != nil {
			logger.Error("Failed to stop messaging client", "error", err)
		}
		if err := messagingClient.Close(); err != nil {
			logger.Error("Failed to close messaging client", "error", err)
		}
	}()

	// Initialize the client manager
	clientManager, err := client.NewClientManager(logger)
	if err != nil {
		logger.Error("Failed to create client manager", "error", err)
		return
	}

	defer clientManager.Stop()

	// Get all available clients
	kubeClients := clientManager.GetClients()

	var managers []*ClusterManagers

	for _, kubeClient := range kubeClients {
		manager, err := setupClusterManagers(messagingClient, kubeClient.Cluster, kubeClient, logger)
		if err != nil {
			logger.Error("Failed to set up managers for cluster",
				"cluster", kubeClient.Cluster,
				"error", err)
			continue
		}
		managers = append(managers, manager)
	}

	// Start all informers
	startAllInformers(managers, logger)

	// Ensure proper cleanup
	defer stopAllInformers(managers, logger)

	<-ctx.Done()
	logger.Info("Context done, shutting down")
}

func setupClusterManagers(msgClient messagetypes.Publisher, clusterID string, client *client.ClusterConfig, logger *slog.Logger) (*ClusterManagers, error) {
	// Send cluster registration using the new package
	err := cluster.PublishConnection(msgClient, client.Cluster, client.Config.Host, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to publish cluster: %w", err)
	}

	return &ClusterManagers{
		Cluster:          client.Cluster,
		NamespaceManager: namespaces.NewManager(clusterID, msgClient, client.Client, logger),
		PodManager:       pods.NewManager(clusterID, msgClient, client.Client, logger),
	}, nil
}

func startAllInformers(managers []*ClusterManagers, logger *slog.Logger) {
	for _, manager := range managers {
		logger.Info("Starting informers", "cluster", manager.Cluster)

		if err := manager.NamespaceManager.StartInformer(); err != nil {
			logger.Error("Failed to start namespace informer",
				"cluster", manager.Cluster,
				"error", err)
		}

		if err := manager.PodManager.StartInformer(); err != nil {
			logger.Error("Failed to start pod informer",
				"cluster", manager.Cluster,
				"error", err)
		}
	}
}

func stopAllInformers(managers []*ClusterManagers, logger *slog.Logger) {
	for _, manager := range managers {
		logger.Info("Stopping informers", "cluster", manager.Cluster)
		manager.NamespaceManager.Stop()
		manager.PodManager.Stop()
	}
}
