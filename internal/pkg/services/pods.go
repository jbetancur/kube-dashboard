package services

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/jbetancur/dashboard/internal/pkg/core"
)

type PodService struct {
	podProvider core.PodProvider
}

func NewPodService(podProvider core.PodProvider) *PodService {
	return &PodService{
		podProvider: podProvider,
	}
}

func (ps *PodService) ListPods(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	namespace := c.Params("namespaceID")

	if clusterID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing cluster ID",
		})
	}

	if namespace == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing namespace",
		})
	}

	// List pods from this specific cluster and namespace
	pods, err := ps.podProvider.ListPods(c.Context(), clusterID, namespace)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to list pods: %v", err),
		})
	}

	return c.JSON(pods)
}

func (ps *PodService) GetPod(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	namespace := c.Params("namespaceID")
	podID := c.Params("podID")

	if clusterID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing cluster ID",
		})
	}

	if namespace == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing namespace",
		})
	}

	if podID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing pod ID",
		})
	}

	pod, err := ps.podProvider.GetPod(c.Context(), clusterID, namespace, podID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to get pod: %v", err),
		})
	}

	return c.JSON(pod)
}

// This is for the websocket pod logs streaming
func (ps *PodService) StreamPodLogs(c *websocket.Conn) {
	clusterID := c.Params("clusterID")
	namespace := c.Params("namespace")
	podName := c.Params("podName")
	// containerName := c.Params("containerName")

	if clusterID == "" || namespace == "" || podName == "" {
		c.WriteJSON(fiber.Map{
			"error": "missing required parameters",
		})
		c.Close()
		return
	}

	// This would need a different implementation since we can't directly use PodManager's StreamPodLogs
	// You'd need to extend the PodProvider interface to support this or handle it differently
	c.WriteJSON(fiber.Map{
		"error": "pod logs streaming not implemented yet",
	})
	c.Close()
}
