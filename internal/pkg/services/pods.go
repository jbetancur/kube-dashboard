package services

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/jbetancur/dashboard/internal/pkg/core"
)

type PodService struct {
	podManager *core.PodManager
}

func NewPodService(podManager *core.PodManager) *PodService {
	return &PodService{podManager: podManager}
}

func (ps *PodService) ListPods(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	namespace := c.Params("namespace")
	pods, err := ps.podManager.ListPods(c.Context(), clusterID, namespace)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	return c.JSON(pods)
}

func (ps *PodService) GetPod(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	namespace := c.Params("namespace")
	podID := c.Params("podID")
	pod, err := ps.podManager.GetPod(c.Context(), clusterID, namespace, podID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	return c.JSON(pod)
}

func (ps *PodService) StreamPodLogs(conn *websocket.Conn) {
	clusterID := conn.Params("clusterID")
	namespace := conn.Params("namespace")
	podName := conn.Params("podName")
	containerName := conn.Params("containerName")
	if err := ps.podManager.StreamPodLogs(context.Background(), clusterID, namespace, podName, containerName, conn); err != nil {
		conn.Close()
	}
}
