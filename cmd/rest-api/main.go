package main

import (
	"log/slog"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/jbetancur/dashboard/internal/pkg/core"
	"github.com/jbetancur/dashboard/internal/pkg/providers"
	"github.com/jbetancur/dashboard/internal/pkg/router"
	"github.com/jbetancur/dashboard/internal/pkg/services"
)

func main() {
	// Initialize slog with a TextHandler (human-readable logs)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger) // Set as the default logger

	logger.Info("Starting application")

	clusterProvider := providers.NewKubeConfigProvider(logger)
	clusterManager := core.NewClusterManager(logger)

	// Discover and add clusters
	clusters, err := clusterProvider.DiscoverClusters()
	if err != nil {
		logger.Error("Error discovering clusters", "error", err)

		return
	}

	// Add clusters to the ClusterManager
	for _, cluster := range clusters {
		if err := clusterManager.AddCluster(cluster.ID, cluster.KubeconfigPath); err != nil {
			logger.Error("Error adding cluster", "clusterID", cluster.ID, "error", err)
		}
	}

	namespaceManager, err := core.NewNamespaceManager(clusterManager, clusters)
	if err != nil {
		logger.Error("Failed to initialize NamespaceManager", "error", err)
		return
	}

	podManager, err := core.NewPodManager(clusterManager, clusters)
	if err != nil {
		logger.Error("Failed to initialize PodManager", "error", err)
		return
	}

	// Initialize services
	clusterService := services.NewClusterService(clusterManager)
	namespaceService := services.NewNamespaceService(namespaceManager)
	podService := services.NewPodService(podManager)

	app := fiber.New()
	router.SetupRoutes(app, clusterService, podService, namespaceService)

	logger.Info("Starting server on :8081")
	if err := app.Listen(":8081"); err != nil {
		logger.Error("Failed to start server", "error", err)
	}
}
