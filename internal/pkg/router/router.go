package router

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/jbetancur/dashboard/internal/pkg/auth"
	"github.com/jbetancur/dashboard/internal/pkg/services"
)

func SetupRoutes(app *fiber.App, clusterService *services.ClusterService, namespaceService *services.NamespaceService, podService *services.PodService, authorizer auth.Authorizer, logger *slog.Logger) {
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// API group with versioning
	api := app.Group("/api/v1")

	// Cluster routes
	api.Get("/clusters", clusterService.ListClusters)
	api.Get("/clusters/:clusterID", clusterService.GetCluster)

	// Namespace routes
	api.Get("/clusters/:clusterID/namespaces",
		auth.AuthMiddleware(),
		auth.RequirePermission(authorizer, logger, auth.ResourceInfo{
			Resource:     "namespaces",
			Verb:         "list",
			ClusterParam: "clusterID",
		}),
		namespaceService.ListNamespaces)

	api.Get("/clusters/:clusterID/namespaces/:namespaceID",
		auth.AuthMiddleware(),
		auth.RequirePermission(authorizer, logger, auth.ResourceInfo{
			Resource:     "namespaces",
			Verb:         "get",
			ClusterParam: "clusterID",
			NameParam:    "namespaceID",
		}),
		namespaceService.GetNamespace)

	// Pod routes
	api.Get("/clusters/:clusterID/namespaces/:namespaceID/pods",
		auth.AuthMiddleware(),
		auth.RequirePermission(authorizer, logger, auth.ResourceInfo{
			Resource:       "pods",
			Verb:           "list",
			ClusterParam:   "clusterID",
			NamespaceParam: "namespaceID",
		}),
		podService.ListPods)

	api.Get("/clusters/:clusterID/namespaces/:namespaceID/pods/:podID",
		auth.AuthMiddleware(),
		auth.RequirePermission(authorizer, logger, auth.ResourceInfo{
			Resource:       "pods",
			Verb:           "get",
			ClusterParam:   "clusterID",
			NamespaceParam: "namespaceID",
			NameParam:      "podID",
		}),
		podService.GetPod)

	// Pod logs via WebSocket
	api.Get("/clusters/:clusterID/namespaces/:namespaceID/pods/:podID/logs/:containerName",
		auth.WebSocketAuthMiddleware(authorizer),
		websocket.New(podService.StreamPodLogs))
}
