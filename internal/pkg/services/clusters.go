package services

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jbetancur/dashboard/internal/pkg/core"
)

type ClusterService struct {
	clusterManager *core.ClusterManager
}

func NewClusterService(clusterManager *core.ClusterManager) *ClusterService {
	return &ClusterService{clusterManager: clusterManager}
}

func (cs *ClusterService) ListClusters(c *fiber.Ctx) error {
	clusters := make([]string, 0)
	cs.clusterManager.Clusters.Range(func(key, value interface{}) bool {
		clusters = append(clusters, key.(string))
		return true
	})

	return c.JSON(clusters)
}

func (cs *ClusterService) GetCluster(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	cluster, exists := cs.clusterManager.GetCluster(clusterID)
	if exists != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Cluster not found"})
	}

	return c.JSON(cluster)
}

func (cs *ClusterService) DeleteCluster(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	if err := cs.clusterManager.RemoveCluster(clusterID); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusOK)
}
