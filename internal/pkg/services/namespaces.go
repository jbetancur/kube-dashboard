package services

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/jbetancur/dashboard/internal/pkg/core"
)

type NamespaceService struct {
	namespaceProvider core.NamespaceProvider
}

func NewNamespaceService(namespaceProvider core.NamespaceProvider) *NamespaceService {
	return &NamespaceService{
		namespaceProvider: namespaceProvider,
	}
}

func (ns *NamespaceService) ListNamespaces(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	if clusterID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing cluster ID",
		})
	}

	// List namespaces from this specific cluster
	namespaces, err := ns.namespaceProvider.ListNamespaces(c.Context(), clusterID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to list namespaces: %v", err),
		})
	}

	return c.JSON(namespaces)
}

func (ns *NamespaceService) GetNamespace(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	namespaceID := c.Params("namespaceID")

	if clusterID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing cluster ID",
		})
	}

	if namespaceID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing namespace ID",
		})
	}

	namespace, err := ns.namespaceProvider.GetNamespace(c.Context(), clusterID, namespaceID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to get namespace: %v", err),
		})
	}

	return c.JSON(namespace)
}
