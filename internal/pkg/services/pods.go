package services

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/jbetancur/dashboard/internal/pkg/core"
)

type PodService struct {
	podManager     *core.PodManager
	clusterManager *core.ClusterManager
}

func NewPodService(podManager *core.PodManager, clusterManager *core.ClusterManager) *PodService {
	return &PodService{
		podManager:     podManager,
		clusterManager: clusterManager,
	}
}

func (ps *PodService) ListPods(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	// namespace := c.Params("namespace")

	pods, err := ps.podManager.ListPods(c.Context(), clusterID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.JSON(pods)

}

func (ps *PodService) GetPod(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	namespace := c.Params("namespace")
	// podID := c.Params("podID")

	pod, err := ps.podManager.GetPod(c.Context(), clusterID, namespace)
	if err != nil {
		return err
	}

	return c.JSON(pod)
}

func (ps *PodService) StreamPodLogs(conn *websocket.Conn) {
	clusterID := conn.Params("clusterID")
	namespace := conn.Params("namespace")
	podName := conn.Params("podName")
	// Use WithReauthentication to handle token expiration
	err := ps.podManager.StreamPodLogs(context.Background(), clusterID, namespace, podName, conn)

	if err != nil {
		// Close the WebSocket connection on error
		conn.WriteMessage(websocket.TextMessage, []byte("Error streaming logs: "+err.Error()))
		conn.Close()
	}
}
