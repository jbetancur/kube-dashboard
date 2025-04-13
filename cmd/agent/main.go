package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jbetancur/dashboard/internal/pkg/client"
	"github.com/jbetancur/dashboard/internal/pkg/cluster"
	"github.com/jbetancur/dashboard/internal/pkg/grpc"
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

	// Initialize the client manager
	clientManager, err := client.NewClientManager(logger)
	if err != nil {
		logger.Error("Failed to create client manager", "error", err)
		return
	}
	defer clientManager.Stop()

	// Get all available clients
	kubeClients := clientManager.GetClients()

	// Initialize the gRPC server
	grpcServer := grpc.NewGRPCServer()
	err = grpcServer.Start(ctx, ":50050")
	if err != nil {
		logger.Error("Failed to start gRPC server", "error", err)
		return
	}

	// Initialize the gRPC client to publish events
	grpcClient := grpc.NewGRPCClient()
	err = grpcClient.Connect(ctx, ":50052") // Connect to REST API's server
	if err != nil {
		logger.Error("Failed to connect to REST API", "error", err)
		return
	}

	// Wait for the REST API to be ready
	err = waitForRESTAPI(logger)
	if err != nil {
		logger.Error("REST API is not ready", "error", err)
		return
	}

	var managers []*ClusterManagers

	for _, kubeClient := range kubeClients {
		manager, err := setupClusterManagers(grpcClient, kubeClient.Cluster, kubeClient, logger)
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

func setupClusterManagers(grpcClient *grpc.GRPCClient, clusterID string, client *client.ClusterConfig, logger *slog.Logger) (*ClusterManagers, error) {
	// Send cluster registration using the new package
	err := cluster.PublishConnection(grpcClient, client.Cluster, client.Config.Host, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to publish cluster: %w", err)
	}

	return &ClusterManagers{
		Cluster:          client.Cluster,
		NamespaceManager: namespaces.NewManager(clusterID, grpcClient, client.Client, logger),
		PodManager:       pods.NewManager(clusterID, grpcClient, client.Client, logger),
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

func waitForRESTAPI(logger *slog.Logger) error {
	healthURL := "http://localhost:8081/health"
	maxAttempts := 10
	delay := 2 * time.Second
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for i := 0; i < maxAttempts; i++ {
		resp, err := client.Get(healthURL)
		if err == nil {
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				logger.Info("REST API is ready")
				return nil
			}

			logger.Warn("REST API returned non-OK status",
				"attempt", i+1,
				"statusCode", resp.StatusCode)
		} else {
			logger.Warn("Failed to connect to REST API",
				"attempt", i+1,
				"error", err)
		}

		time.Sleep(delay)
	}

	return fmt.Errorf("REST API is not ready after %d attempts", maxAttempts)
}
