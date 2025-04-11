package services

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/jbetancur/dashboard/internal/pkg/core"
)

type NamespaceService struct {
	clusterManager   *core.ClusterManager
	namespaceManager *core.NamespaceManager
}

func NewNamespaceService(namespaceManager *core.NamespaceManager, clusterManager *core.ClusterManager) *NamespaceService {
	return &NamespaceService{
		namespaceManager: namespaceManager,
		clusterManager:   clusterManager,
	}
}

func (ns *NamespaceService) ListNamespaces(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	if clusterID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing cluster ID",
		})
	}

	// Get the cluster connection
	_, err := ns.clusterManager.GetCluster(clusterID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fmt.Sprintf("cluster not found: %v", err),
		})
	}

	// List namespaces from this specific cluster
	namespaces, err := ns.namespaceManager.ListNamespaces(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to list namespaces: %v", err),
		})
	}

	return c.JSON(namespaces)
}

func (ns *NamespaceService) GetNamespace(c *fiber.Ctx) error {
	// clusterID := c.Params("clusterID")
	namespaceID := c.Params("namespaceID")

	namespace, err := ns.namespaceManager.GetNamespace(c.Context(), namespaceID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.JSON(namespace)
}
