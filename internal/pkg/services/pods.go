package services

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/jbetancur/dashboard/internal/pkg/core"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
	namespace := c.Params("namespace")

	err := ps.clusterManager.WithReauthentication(clusterID, func(client *kubernetes.Clientset) error {
		pods, err := client.CoreV1().Pods(namespace).List(c.Context(), metav1.ListOptions{})
		if err != nil {
			return err
		}

		return c.JSON(pods.Items)
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return nil
}

func (ps *PodService) GetPod(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	namespace := c.Params("namespace")
	podID := c.Params("podID")

	err := ps.clusterManager.WithReauthentication(clusterID, func(client *kubernetes.Clientset) error {
		pod, err := ps.podManager.GetPod(c.Context(), clusterID, namespace, podID)
		if err != nil {
			return err
		}

		return c.JSON(pod)
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return nil
}

func (ps *PodService) StreamPodLogs(conn *websocket.Conn) {
	clusterID := conn.Params("clusterID")
	namespace := conn.Params("namespace")
	podName := conn.Params("podName")
	containerName := conn.Params("containerName")

	// Use WithReauthentication to handle token expiration
	err := ps.clusterManager.WithReauthentication(clusterID, func(client *kubernetes.Clientset) error {
		// Stream logs using the authenticated client
		return ps.podManager.StreamPodLogs(context.Background(), clusterID, namespace, podName, containerName, conn)
	})

	if err != nil {
		// Close the WebSocket connection on error
		conn.WriteMessage(websocket.TextMessage, []byte("Error streaming logs: "+err.Error()))
		conn.Close()
	}
}
