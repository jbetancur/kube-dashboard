package services

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jbetancur/dashboard/internal/pkg/core"
)

type NamespaceService struct {
	namespaceManager *core.NamespaceManager
}

func NewNamespaceService(namespaceManager *core.NamespaceManager) *NamespaceService {
	return &NamespaceService{namespaceManager: namespaceManager}
}

func (ns *NamespaceService) ListNamespaces(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	namespaces, err := ns.namespaceManager.ListNamespaces(c.Context(), clusterID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.JSON(namespaces)
}

func (ns *NamespaceService) GetNamespace(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	namespaceID := c.Params("namespaceID")
	namespace, err := ns.namespaceManager.GetNamespace(c.Context(), clusterID, namespaceID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.JSON(namespace)
}
