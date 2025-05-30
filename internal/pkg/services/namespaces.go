package services

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/jbetancur/dashboard/internal/pkg/assets/namespaces"

	"github.com/jbetancur/dashboard/internal/pkg/store"

	corev1 "k8s.io/api/core/v1"
)

type NamespaceService struct {
	BaseService
	provider *namespaces.MultiClusterNamespaceProvider
	store    store.Repository
}

// NewNamespaceService creates a new namespace service
func NewNamespaceService(provider *namespaces.MultiClusterNamespaceProvider, store store.Repository,
	logger *slog.Logger) *NamespaceService {
	return &NamespaceService{
		BaseService: BaseService{Logger: logger},
		provider:    provider,
		store:       store,
	}
}

func (s *NamespaceService) ListNamespaces(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	if clusterID == "" {
		return s.BadRequest(c, "missing cluster ID")
	}

	s.Logger.Debug("Listing namespaces fom data store", "clusterID", clusterID)

	// Use MongoDB to list namespaces instead of the provider
	var namespaces []corev1.Namespace
	if err := s.store.List(c.Context(), clusterID, "", "Namespace", &namespaces); err != nil {
		s.Logger.Error("Failed to list namespaces fom data store", "clusterID", clusterID, "error", err)

		// // Fallback to direct API call if MongoDB fails
		// directNamespaces, directErr := s.provider.ListNamespaces(c.Context(), clusterID)
		// if directErr != nil {
		// 	return s.Error(c, fiber.StatusInternalServerError, "failed to list namespaces: %v", err)
		// }
		// return c.JSON(directNamespaces)
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

	s.Logger.Debug("Getting namespace fom data store", "clusterID", clusterID, "namespaceID", namespaceID)

	// Use MongoDB to get a namespace instead of the provider
	var namespace corev1.Namespace
	if err := s.store.Get(c.Context(), clusterID, "", "Namespace", namespaceID, &namespace); err != nil {
		s.Logger.Error("Failed to get namespace fom data store",
			"clusterID", clusterID,
			"namespaceID", namespaceID,
			"error", err)

		// // Fallback to direct API call if MongoDB fails
		// directNamespace, directErr := s.provider.GetNamespace(c.Context(), clusterID, namespaceID)
		// if directErr != nil {
		// 	return s.Error(c, fiber.StatusInternalServerError, "failed to get namespace: %v", err)
		// }
		// if directNamespace == nil {
		// 	return s.NotFound(c, "Namespace", namespaceID)
		// }
		// return c.JSON(directNamespace)
	}

	return c.JSON(&namespace)
}
