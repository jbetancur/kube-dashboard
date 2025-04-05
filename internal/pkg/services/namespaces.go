package services

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jbetancur/dashboard/internal/pkg/core"
	"k8s.io/client-go/kubernetes"
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

	// Use WithReauthentication to handle token expiration
	err := ns.clusterManager.WithReauthentication(clusterID, func(client *kubernetes.Clientset) error {
		namespaces, err := ns.namespaceManager.ListNamespaces(c.Context(), clusterID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}

		return c.JSON(namespaces)
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return nil
}

func (ns *NamespaceService) GetNamespace(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	namespaceID := c.Params("namespaceID")

	// Use WithReauthentication to handle token expiration
	err := ns.clusterManager.WithReauthentication(clusterID, func(client *kubernetes.Clientset) error {
		namespace, err := ns.namespaceManager.GetNamespace(c.Context(), clusterID, namespaceID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}

		return c.JSON(namespace)
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return nil
}
