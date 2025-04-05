package main

import (
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

	// Discover clusters
	clusters, err := clusterProvider.DiscoverClusters()
	if err != nil {
		logger.Error("Error discovering clusters", "error", err)
		return
	}

	clusterManager := core.NewClusterManager(logger, clusterProvider)

	// Add clusters to the ClusterManager
	for _, cluster := range clusters {
		if err := clusterManager.RegisterCluster(cluster.ID); err != nil {
			logger.Error("Error adding cluster", "clusterID", cluster.ID, "error", err)
		}
	}

	// Initialize the message queue (e.g., gRPC, Kafka, RabbitMQ)
	messageQueue := messaging.NewGRPCMessageQueue()
	defer messageQueue.Close()

	// Create the shared EventPublisher
	eventPublisher := core.NewEventPublisher(messageQueue)

	// Register subscribers for topics
	err = messageQueue.Subscribe("pod_events", func(message []byte) error {
		// logger.Info("Received pod event", "message", string(message))
		// Process the pod event (e.g., store it in a database)
		return nil
	})
	if err != nil {
		logger.Error("Failed to subscribe to pod_events", "error", err)
		return
	}

	err = messageQueue.Subscribe("namespace_events", func(message []byte) error {
		// logger.Info("Received namespace event", "message", string(message))
		// Process the namespace event (e.g., store it in a database)
		return nil
	})
	if err != nil {
		logger.Error("Failed to subscribe to namespace_events", "error", err)
		return
	}

	namespaceManager, err := core.NewNamespaceManager(eventPublisher, clusterManager, clusters)
	if err != nil {
		logger.Error("Failed to initialize NamespaceManager", "error", err)
		return
	}

	podManager, err := core.NewPodManager(eventPublisher, clusterManager, clusters)
	if err != nil {
		logger.Error("Failed to initialize PodManager", "error", err)
		return
	}

	// Initialize services
	clusterService := services.NewClusterService(clusterManager)
	namespaceService := services.NewNamespaceService(namespaceManager, clusterManager)
	podService := services.NewPodService(podManager, clusterManager)

	app := fiber.New()
	router.SetupRoutes(app, clusterService, podService, namespaceService)

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
