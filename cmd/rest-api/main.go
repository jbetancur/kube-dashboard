package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"plugin"

	"github.com/gofiber/fiber/v2"
	"github.com/jbetancur/dashboard/internal/pkg/assets/configmaps"
	"github.com/jbetancur/dashboard/internal/pkg/assets/namespaces"
	"github.com/jbetancur/dashboard/internal/pkg/assets/pods"
	"github.com/jbetancur/dashboard/internal/pkg/auth"
	"github.com/jbetancur/dashboard/internal/pkg/cluster"
	"github.com/jbetancur/dashboard/internal/pkg/config"
	"github.com/jbetancur/dashboard/internal/pkg/providers"
	"github.com/jbetancur/dashboard/internal/pkg/router"
	"github.com/jbetancur/dashboard/internal/pkg/services"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	// Initialize slog with a TextHandler (human-readable logs)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)
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

	//
	store, err := config.Store(ctx, logger)
	if err != nil {
		logger.Error("Failed to initialize store", "error", err)

		return
	}

	// Stop and close the store
	defer func() {
		if err := store.Close(ctx); err != nil {
			logger.Error("Failed to close MongoDB store", "error", err)
		}
	}()

	// Initialize the messaging client for bidirectional communication
	messagingClient, err := config.StartMessageClients(ctx, logger)
	if err != nil {
		logger.Error("Failed to start message clients", "error", err)

		return
	}

	// Stop and close the messaging client
	defer func() {
		if err := messagingClient.Stop(); err != nil {
			logger.Error("Failed to stop messaging client", "error", err)
			return
		}
		if err := messagingClient.Close(); err != nil {
			logger.Error("Failed to close messaging client", "error", err)
			return
		}
	}()

	clusterManager := cluster.NewManager(ctx, logger, clusterProvider)

	// Create authorizer using your cluster manager
	k8sAuthorizer := auth.NewK8sAuthorizer(clusterManager, logger)
	config.SetupSubscriptions(ctx, messagingClient, store, clusterManager, logger)

	// Initialize services
	clusterService := services.NewClusterService(clusterManager, store, logger)

	// Create a multi-cluster namespace provider (no informers)
	namespaceProvider := namespaces.NewNamespaceProvider(clusterManager)
	namespaceService := services.NewNamespaceService(namespaceProvider, store, logger)

	podProvider := pods.NewPodProvider(clusterManager)
	podService := services.NewPodService(podProvider, store, logger)

	configMapProvider := configmaps.NewConfigMapProvider(clusterManager)
	configMapService := services.NewConfigMapService(configMapProvider, store, logger)

	app := fiber.New()
	router.SetupRoutes(
		app,
		clusterService,
		namespaceService,
		podService,
		configMapService,
		k8sAuthorizer,
		logger,
	)

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
