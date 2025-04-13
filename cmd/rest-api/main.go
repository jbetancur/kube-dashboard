package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"plugin"

	"github.com/gofiber/fiber/v2"
	"github.com/jbetancur/dashboard/internal/pkg/cluster"
	"github.com/jbetancur/dashboard/internal/pkg/config"
	"github.com/jbetancur/dashboard/internal/pkg/messaging"
	"github.com/jbetancur/dashboard/internal/pkg/mongo"
	"github.com/jbetancur/dashboard/internal/pkg/providers"
	"github.com/jbetancur/dashboard/internal/pkg/resources"
	"github.com/jbetancur/dashboard/internal/pkg/resources/namespaces"
	"github.com/jbetancur/dashboard/internal/pkg/resources/pods"
	"github.com/jbetancur/dashboard/internal/pkg/router"
	"github.com/jbetancur/dashboard/internal/pkg/services"
	corev1 "k8s.io/api/core/v1"
)

func main() {
	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	// In your main.go or initialization
	store, err := mongo.NewStore(ctx, "mongodb://localhost:27017", "k8s-dashboard", logger)
	if err != nil {
		logger.Error("Failed to create MongoDB store", "error", err)
		return
	}
	defer store.Close(ctx)

	// Initialize the gRPC client
	grpcClient := messaging.NewGRPCClient()
	err = grpcClient.Connect(ctx, ":50050")
	if err != nil {
		logger.Error("Failed to connect to gRPC server", "error", err)
		return
	}

	// Initialize the gRPC server to handle incoming events
	grpcServer := messaging.NewGRPCServer()
	err = grpcServer.Start(ctx, ":50052") // Use a different port
	if err != nil {
		logger.Error("Failed to start gRPC server", "error", err)
		return
	}

	logger.Info("Connected to gRPC server on :50051")
	// Create a cluster manager with context for lifecycle management
	clusterManager := cluster.NewManager(ctx, logger, clusterProvider)

	// Subscribe to cluster_registered events
	grpcServer.Subscribe("cluster_registered", func(message []byte) error {
		var payload cluster.ConnectionPayload
		err := json.Unmarshal(message, &payload)
		if err != nil {
			logger.Error("Failed to unmarshal cluster connection event", "error", err)
			return err
		}

		// Register the cluster
		err = clusterManager.Register(payload.ClusterName, payload.APIURL)
		if err != nil {
			logger.Error("Failed to register cluster", "error", err)
			return err
		}

		// var eventData struct {
		// 	ClusterID string     `json:"cluster_id"`
		// 	Resource  corev1.Pod `json:"resource"`
		// }

		// if err := json.Unmarshal(message, &eventData); err != nil {
		// 	logger.Error("Failed to unmarshal namespace event", "error", err)
		// 	return err
		// }

		// // Store in MongoDB
		// if err := store.Save(ctx, "system", clusterResource); err != nil {
		// 	logger.Error("Failed to store cluster", "error", err)
		// 	return err
		// }

		logger.Info("Stored cluster from event",
			"name", payload.ClusterName,
			"api_url", payload.APIURL)
		return nil
	})

	// Add handlers for events
	grpcServer.Subscribe("pod_added", func(message []byte) error {
		var payload resources.ResourcePayload[corev1.Pod]
		if err := json.Unmarshal(message, &payload); err != nil {
			logger.Error("Failed to unmarshal namespace event", "error", err)
			return err
		}

		if err := store.Save(ctx, payload.ClusterID, &payload.Resource); err != nil {
			logger.Error("Failed to store pod", "error", err)
			return err
		}

		logger.Debug("Stored pod from event", "name", payload.Resource.Name, "namespace", payload.Resource.Namespace, "cluster", payload.ClusterID)

		return nil
	})

	grpcServer.Subscribe("namespace_added", func(message []byte) error {
		var payload resources.ResourcePayload[corev1.Namespace]

		if err := json.Unmarshal(message, &payload); err != nil {
			logger.Error("Failed to unmarshal namespace event", "error", err)
			return err
		}

		if err := store.Save(ctx, payload.ClusterID, &payload.Resource); err != nil {
			logger.Error("Failed to store namespace", "error", err)
			return err
		}

		logger.Info("Stored namespace from event", "name", payload.Resource.Name, "cluster", payload.ClusterID)

		return nil
	})

	// Initialize services
	clusterService := services.NewClusterService(clusterManager, logger)

	// Create a multi-cluster namespace provider (no informers)
	namespaceProvider := namespaces.NewMultiClusterNamespaceProvider(clusterManager)
	namespaceService := services.NewNamespaceService(namespaceProvider, store, logger)

	podProvider := pods.NewMultiClusterPodProvider(clusterManager)
	podService := services.NewPodService(podProvider, store, logger)

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
