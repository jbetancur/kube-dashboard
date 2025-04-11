package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"plugin"

	"github.com/gofiber/fiber/v2"
	"github.com/jbetancur/dashboard/internal/pkg/config"
	"github.com/jbetancur/dashboard/internal/pkg/core"
	"github.com/jbetancur/dashboard/internal/pkg/messaging"
	"github.com/jbetancur/dashboard/internal/pkg/providers"
	"github.com/jbetancur/dashboard/internal/pkg/router"
	"github.com/jbetancur/dashboard/internal/pkg/services"
)

func main() {
	// Initialize slog with a TextHandler (human-readable logs)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger) // Set as the default logger
	logger.Info("Starting application")

	// Load configuration
	appConfig, err := config.LoadConfig("config.yaml")
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		return
	}

	// Load provider plugins
	var clusterProvider providers.Provider
	for _, providerConfig := range appConfig.Providers {
		clusterProvider, err = loadProviderPlugin(providerConfig.Path, providerConfig.Config, logger)
		if err != nil {
			logger.Error("Failed to load provider plugin", "name", providerConfig.Name, "error", err)
			return
		}
		logger.Info("Loaded provider plugin", "name", providerConfig.Name)
	}

	// // Discover clusters
	// clusters, err := clusterProvider.DiscoverClusters()
	// if err != nil {
	// 	logger.Error("Error discovering clusters", "error", err)
	// 	return
	// }

	// Initialize the gRPC client
	grpcClient := messaging.NewGRPCClient()
	err = grpcClient.Connect("127.0.0.1:50051")
	if err != nil {
		logger.Error("Failed to connect to gRPC server", "error", err)
		return
	}

	// Initialize the gRPC server to handle incoming events
	grpcServer := messaging.NewGRPCServer()
	err = grpcServer.Start("127.0.0.1:50052") // Use a different port
	if err != nil {
		logger.Error("Failed to start gRPC server", "error", err)
		return
	}

	logger.Info("Connected to gRPC server on 127.0.0.1:50051")
	clusterManager := core.NewClusterManager(logger, clusterProvider)
	// Subscribe to cluster_registered
	grpcServer.AddHandler("cluster_registered", func(message []byte) error {
		var payload core.ClusterConnectionPayload
		err := json.Unmarshal(message, &payload)
		if err != nil {
			logger.Error("Failed to unmarshal cluster connection event", "error", err)
			return err
		}

		// Add the cluster to the ClusterManager
		err = clusterManager.Register(payload.ClusterName, payload.APIURL)
		if err != nil {
			logger.Error("Failed to add cluster to ClusterManager", "error", err)
			return err
		}

		return nil
	})
	if err != nil {
		logger.Error("Failed to subscribe to cluster_registered", "error", err)
		return
	}

	// Add handlers for events
	grpcServer.AddHandler("pod_added", func(message []byte) error {
		// logger.Info("Received pod event", "message", string(message))
		return nil
	})

	grpcServer.AddHandler("namespace_added", func(message []byte) error {
		// logger.Info("Received namespace event", "message", string(message))
		return nil
	})

	// Initialize services
	clusterService := services.NewClusterService(clusterManager)

	// Create a multi-cluster namespace provider (no informers)
	namespaceProvider := core.NewMultiClusterNamespaceProvider(clusterManager)
	namespaceService := services.NewNamespaceService(namespaceProvider)

	podProvider := core.NewMultiClusterPodProvider(clusterManager)
	podService := services.NewPodService(podProvider)

	app := fiber.New()
	router.SetupRoutes(app, clusterService, namespaceService, podService)

	logger.Info("Starting server on :8081")
	if err := app.Listen(":8081"); err != nil {
		logger.Error("Failed to start server", "error", err)
	}
}

func loadProviderPlugin(path string, config map[string]string, logger *slog.Logger) (providers.Provider, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin: %w", err)
	}

	symbol, err := p.Lookup("New")
	if err != nil {
		return nil, fmt.Errorf("failed to find 'New' function in plugin: %w", err)
	}

	newFunc, ok := symbol.(func(map[string]string, *slog.Logger) providers.Provider)
	if !ok {
		return nil, fmt.Errorf("invalid 'New' function signature in plugin")
	}

	return newFunc(config, logger), nil
}
