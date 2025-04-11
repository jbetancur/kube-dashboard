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

	"github.com/jbetancur/dashboard/internal/pkg/core"
	"github.com/jbetancur/dashboard/internal/pkg/messaging"
)

type ClusterManagers struct {
	Cluster          string
	NamespaceManager *core.NamespaceManager
	PodManager       *core.PodManager
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

	// Load Kubernetes configuration
	kubeClient, err := core.NewKubeClient(logger)
	if err != nil {
		logger.Error("Failed to create Kubernetes client", "error", err)
		return
	}

	// Initialize the gRPC server
	grpcServer := messaging.NewGRPCServer()
	err = grpcServer.Start(":50051")
	if err != nil {
		logger.Error("Failed to start gRPC server", "error", err)
		return
	}

	// Initialize the gRPC client to publish events
	grpcClient := messaging.NewGRPCClient()
	err = grpcClient.Connect(":50052") // Connect to REST API's server
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

	for _, client := range kubeClient {
		manager, err := setupClusterManagers(grpcClient, client, logger)
		if err != nil {
			logger.Error("Failed to set up managers for cluster",
				"cluster", client.Cluster,
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
	logger.Info("Shutting down agent...")

	// Give time for clean shutdown
	shutdownTimeout := time.NewTimer(10 * time.Second)
	shutdownComplete := make(chan struct{})

	go func() {
		// Perform cleanup
		if grpcServer != nil {
			grpcServer.Stop()
		}

		close(shutdownComplete)
	}()

	select {
	case <-shutdownComplete:
		logger.Info("Graceful shutdown completed")
	case <-shutdownTimeout.C:
		logger.Warn("Shutdown timed out, forcing exit")
	}
}

func setupClusterManagers(grpcClient *messaging.GRPCClient, client *core.ClusterConfig, logger *slog.Logger) (*ClusterManagers, error) {
	// Send cluster registration
	err := core.PublishCluster(grpcClient, client.Cluster, client.Config.Host, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to publish cluster: %w", err)
	}

	return &ClusterManagers{
		Cluster:          client.Cluster,
		NamespaceManager: core.NewNamespaceManager(grpcClient, client.Client),
		PodManager:       core.NewPodManager(grpcClient, client.Client),
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
