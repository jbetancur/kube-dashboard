package services

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/jbetancur/dashboard/internal/pkg/core"
)

// ClusterService handles cluster-related operations
type ClusterService struct {
	clusterManager *core.ClusterManager
}

// NewClusterService creates a new ClusterService
func NewClusterService(clusterManager *core.ClusterManager) *ClusterService {
	return &ClusterService{
		clusterManager: clusterManager,
	}
}

// ListClusters returns a list of all registered clusters
func (cs *ClusterService) ListClusters(c *fiber.Ctx) error {
	clusters := cs.clusterManager.ListClusters()
	return c.JSON(fiber.Map{
		"clusters": clusters,
	})
}

// GetCluster retrieves a specific cluster connection
func (cs *ClusterService) GetCluster(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")

	cluster, err := cs.clusterManager.GetCluster(clusterID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to get cluster %s: %v", clusterID, err),
		})
	}

	return c.JSON(fiber.Map{
		"clusterID": cluster.ID,
	})
}
