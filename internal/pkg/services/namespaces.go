package services

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/jbetancur/dashboard/internal/pkg/resources/namespaces"
)

type NamespaceService struct {
	BaseService
	provider *namespaces.MultiClusterNamespaceProvider
}

func NewNamespaceService(provider *namespaces.MultiClusterNamespaceProvider, logger *slog.Logger) *NamespaceService {
	return &NamespaceService{
		BaseService: BaseService{Logger: logger},
		provider:    provider,
	}
}

func (s *NamespaceService) ListNamespaces(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	if clusterID == "" {
		return s.BadRequest(c, "missing cluster ID")
	}

	s.Logger.Info("Listing namespaces", "clusterID", clusterID)

	namespaces, err := s.provider.ListNamespaces(c.Context(), clusterID)
	if err != nil {
		return s.Error(c, fiber.StatusInternalServerError, "failed to list namespaces: %v", err)
	}

	return c.JSON(namespaces)
}

func (s *NamespaceService) GetNamespace(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	namespaceID := c.Params("namespaceID")

	if clusterID == "" {
		return s.BadRequest(c, "missing cluster ID")
	}

	if namespaceID == "" {
		return s.BadRequest(c, "missing namespace ID")
	}

	s.Logger.Info("Getting namespace", "clusterID", clusterID, "namespaceID", namespaceID)

	namespace, err := s.provider.GetNamespace(c.Context(), clusterID, namespaceID)
	if err != nil {
		return s.Error(c, fiber.StatusInternalServerError, "failed to get namespace: %v", err)
	}

	if namespace == nil {
		return s.NotFound(c, "Namespace", namespaceID)
	}

	return c.JSON(namespace)
}
