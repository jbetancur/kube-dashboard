package services

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/jbetancur/dashboard/internal/pkg/assets/configmaps"

	"github.com/jbetancur/dashboard/internal/pkg/store"

	corev1 "k8s.io/api/core/v1"
)

type ConfigMapService struct {
	BaseService
	provider *configmaps.ConfigMapProvider
	store    store.Repository
}

// NewConfigMapService creates a new config map service
func NewConfigMapService(provider *configmaps.ConfigMapProvider, store store.Repository,
	logger *slog.Logger) *ConfigMapService {
	return &ConfigMapService{
		BaseService: BaseService{Logger: logger},
		provider:    provider,
		store:       store,
	}
}

func (s *ConfigMapService) ListConfigMaps(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	if clusterID == "" {
		return s.BadRequest(c, "missing cluster ID")
	}

	s.Logger.Debug("Listing config maps fom data store", "clusterID", clusterID)

	// Use MongoDB to list config maps instead of the provider
	var configMaps []corev1.ConfigMap
	if err := s.store.List(c.Context(), clusterID, "", "ConfigMap", &configMaps); err != nil {
		s.Logger.Error("Failed to list config maps fom data store", "clusterID", clusterID, "error", err)

		// // Fallback to direct API call if MongoDB fails
		// directConfigMaps, directErr := s.provider.ListConfigMaps(c.Context(), clusterID)
		// if directErr != nil {
		// 	return s.Error(c, fiber.StatusInternalServerError, "failed to list config maps: %v", err)
		// }
		// return c.JSON(directConfigMaps)
	}

	return c.JSON(configMaps)
}

func (s *ConfigMapService) GetConfigMap(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	configMapID := c.Params("configMapID")

	if clusterID == "" {
		return s.BadRequest(c, "missing cluster ID")
	}

	if configMapID == "" {
		return s.BadRequest(c, "missing config map ID")
	}

	s.Logger.Debug("Getting config map fom data store", "clusterID", clusterID, "configMapID", configMapID)

	// Use MongoDB to get a config map instead of the provider
	var configMap corev1.ConfigMap
	if err := s.store.Get(c.Context(), clusterID, "", "ConfigMap", configMapID, &configMap); err != nil {
		s.Logger.Error("Failed to get config map fom data store",
			"clusterID", clusterID,
			"configMapID", configMapID,
			"error", err)

		// // Fallback to direct API call if MongoDB fails
		// directConfigMap, directErr := s.provider.GetConfigMap(c.Context(), clusterID, configMapID)
		// if directErr != nil {
		// 	return s.Error(c, fiber.StatusInternalServerError, "failed to get config map: %v", err)
		// }
		// if directConfigMap == nil {
		// 	return s.NotFound(c, "ConfigMap", configMapID)
		// }
		// return c.JSON(directNamespace)
	}

	return c.JSON(&configMap)
}
