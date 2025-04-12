package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/jbetancur/dashboard/internal/pkg/services"
)

func SetupRoutes(app *fiber.App, clusterService *services.ClusterService, namespaceService *services.NamespaceService, podService *services.PodService) {
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// API group with versioning
	api := app.Group("/api/v1")

	// Cluster routes
	api.Get("/clusters", clusterService.ListClusters)
	api.Get("/clusters/:clusterID", clusterService.GetCluster)

	// Namespace routes
	api.Get("/clusters/:clusterID/namespaces", namespaceService.ListNamespaces)
	api.Get("/clusters/:clusterID/namespaces/:namespaceID", namespaceService.GetNamespace)

	// Pod routes
	api.Get("/clusters/:clusterID/namespaces/:namespaceID/pods", podService.ListPods)
	api.Get("/clusters/:clusterID/namespaces/:namespaceID/pods/:podID", podService.GetPod)

	// Pod logs via WebSocket
	api.Get("/clusters/:clusterID/namespaces/:namespaceID/pods/:podID/logs/:containerName?", websocket.New(podService.StreamPodLogs))
}
