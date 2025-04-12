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

	// Get cluster info from the manager
	clusters := s.manager.ListClusters()

	// The JSON response will now automatically include the ID, Name, and ApiURL fields
	return c.JSON(clusters)
}

func (s *ClusterService) GetCluster(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	if clusterID == "" {
		return s.BadRequest(c, "missing cluster ID")
	}

	s.Logger.Info("Getting cluster", "clusterID", clusterID)

	// Get the cluster
	conn, err := s.manager.GetCluster(clusterID)
	if err != nil {
		return s.NotFound(c, "Cluster", clusterID)
	}

	// Get cluster health status
	healthy, err := conn.GetHealthStatus()
	healthStatus := "unknown"
	if err == nil {
		if healthy {
			healthStatus = "healthy"
		} else {
			healthStatus = "unhealthy"
		}
	}

	// Get API URL
	apiUrl := ""
	if conn.Client != nil && conn.Client.RESTClient() != nil && conn.Client.RESTClient().Get() != nil {
		apiUrl = conn.Client.RESTClient().Get().URL().Host
	}

	// Format the response with the same structure as ListClusters
	response := cluster.ClusterInfo{
		ID:     clusterID,
		Name:   clusterID, // Using ID as name for now
		ApiURL: apiUrl,
		Status: healthStatus, // Additional field for single cluster view
	}

	return c.JSON(response)
}
