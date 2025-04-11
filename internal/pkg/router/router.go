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
	// Cluster routes
	app.Get("/clusters", clusterService.ListClusters)
	app.Get("/clusters/:clusterID", clusterService.GetCluster)

	// Namespace routes
	app.Get("/clusters/:clusterID/namespaces", namespaceService.ListNamespaces)
	app.Get("/clusters/:clusterID/namespaces/:namespaceID", namespaceService.GetNamespace)

	// Pod routes
	app.Get("/clusters/:clusterID/namespaces/:namespaceID/pods", podService.ListPods)
	app.Get("/clusters/:clusterID/namespaces/:namespaceID/pods/:podID", podService.GetPod)
	app.Get("/clusters/:clusterID/namespaces/:namespaceID/:podName/:containerName?", websocket.New(podService.StreamPodLogs))
}
