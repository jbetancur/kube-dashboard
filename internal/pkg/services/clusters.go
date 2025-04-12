package services

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/jbetancur/dashboard/internal/pkg/cluster"
)

type ClusterService struct {
	BaseService
	manager *cluster.Manager
}

func NewClusterService(manager *cluster.Manager, logger *slog.Logger) *ClusterService {
	return &ClusterService{
		BaseService: BaseService{Logger: logger},
		manager:     manager,
	}
}

func (s *ClusterService) ListClusters(c *fiber.Ctx) error {
	s.Logger.Info("Listing clusters")

	clusterIDs := s.manager.ListClusters()

	// Format the response
	response := make([]map[string]interface{}, len(clusterIDs))
	for i, id := range clusterIDs {
		response[i] = map[string]interface{}{
			"id":   id,
			"name": id, // Using ID as name for now
		}
	}

	return c.JSON(response)
}

func (s *ClusterService) GetCluster(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	if clusterID == "" {
		return s.BadRequest(c, "missing cluster ID")
	}

	s.Logger.Info("Getting cluster", "clusterID", clusterID)

	// Get the cluster
	cluster, err := s.manager.GetCluster(clusterID)
	if err != nil {
		return s.NotFound(c, "Cluster", clusterID)
	}

	// Get cluster health status
	healthy, err := cluster.GetHealthStatus()
	healthStatus := "unknown"
	if err == nil {
		if healthy {
			healthStatus = "healthy"
		} else {
			healthStatus = "unhealthy"
		}
	}

	// Format the response
	response := map[string]interface{}{
		"id":     clusterID,
		"name":   clusterID, // Using ID as name for now
		"status": healthStatus,
	}

	return c.JSON(response)
}
