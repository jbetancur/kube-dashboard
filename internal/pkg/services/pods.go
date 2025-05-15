package services

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/jbetancur/dashboard/internal/pkg/assets/pods"
	"github.com/jbetancur/dashboard/internal/pkg/store"
	corev1 "k8s.io/api/core/v1"
)

type PodService struct {
	BaseService
	provider *pods.MultiClusterPodProvider
	store    store.Repository // Add MongoDB store
}

func NewPodService(provider *pods.MultiClusterPodProvider, store store.Repository, logger *slog.Logger) *PodService {
	return &PodService{
		BaseService: BaseService{Logger: logger},
		provider:    provider,
		store:       store, // Store the MongoDB repository
	}
}

func (s *PodService) ListPods(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	namespaceID := c.Params("namespaceID")

	if clusterID == "" {
		return s.BadRequest(c, "missing cluster ID")
	}

	if namespaceID == "" {
		return s.BadRequest(c, "missing namespace ID")
	}

	s.Logger.Info("Debug pods fom data store",
		"clusterID", clusterID,
		"namespaceID", namespaceID)

	// Use MongoDB to list pods instead of the provider
	var pods []corev1.Pod
	if err := s.store.List(c.Context(), clusterID, namespaceID, "Pod", &pods); err != nil {
		s.Logger.Error("Failed to list pods fom data store",
			"clusterID", clusterID,
			"namespaceID", namespaceID,
			"error", err)

		// // Fallback to direct API call if MongoDB fails
		// directPods, directErr := s.provider.ListPods(c.Context(), clusterID, namespaceID)
		// if directErr != nil {
		// 	return s.Error(c, fiber.StatusInternalServerError, "failed to list pods: %v", err)
		// }
		// return c.JSON(directPods)
	}

	return c.JSON(pods)
}

func (s *PodService) GetPod(c *fiber.Ctx) error {
	clusterID := c.Params("clusterID")
	namespaceID := c.Params("namespaceID")
	podID := c.Params("podID")

	if clusterID == "" {
		return s.BadRequest(c, "missing cluster ID")
	}

	if namespaceID == "" {
		return s.BadRequest(c, "missing namespace ID")
	}

	if podID == "" {
		return s.BadRequest(c, "missing pod ID")
	}

	s.Logger.Debug("Getting pod fom data store",
		"clusterID", clusterID,
		"namespaceID", namespaceID,
		"podID", podID)

	// Use MongoDB to get a pod instead of the provider
	var pod corev1.Pod
	if err := s.store.Get(c.Context(), clusterID, namespaceID, "Pod", podID, &pod); err != nil {
		s.Logger.Error("Failed to get pod fom data store",
			"clusterID", clusterID,
			"namespaceID", namespaceID,
			"podID", podID,
			"error", err)

		// // Fallback to direct API call if MongoDB fails
		// directPod, directErr := s.provider.GetPod(c.Context(), clusterID, namespaceID, podID)
		// if directErr != nil {
		// 	return s.Error(c, fiber.StatusInternalServerError, "failed to get pod: %v", err)
		// }
		// if directPod == nil {
		// 	return s.NotFound(c, "Pod", podID)
		// }
		// return c.JSON(directPod)
	}

	return c.JSON(&pod)
}

// StreamPodLogs still needs to use the direct API as we can't stream logs fom data store
func (s *PodService) StreamPodLogs(c *websocket.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clusterID := c.Params("clusterID")
	namespaceID := c.Params("namespaceID")
	podID := c.Params("podID")
	containerName := c.Params("containerName")

	if clusterID == "" || namespaceID == "" || podID == "" {
		s.sendLogError(c, "Missing required parameters")
		return
	}

	// Parse query params for log options
	tailLines := 100 // Default
	if tailParam := c.Query("tail"); tailParam != "" {
		if val, err := strconv.Atoi(tailParam); err == nil && val > 0 {
			tailLines = val
		}
	}

	// Use the direct provider for streaming logs
	logStream, err := s.provider.GetPodLogs(ctx, clusterID, namespaceID, podID, containerName, int64(tailLines))
	if err != nil {
		s.sendLogError(c, fmt.Sprintf("Failed to get pod logs: %v", err))
		return
	}

	defer func() {
		if err := logStream.Close(); err != nil {
			s.Logger.Error("Failed to close log stream", "error", err)
		}
	}()

	s.Logger.Info("Streaming pod logs",
		"clusterID", clusterID,
		"namespaceID", namespaceID,
		"podID", podID,
		"container", containerName)

	reader := bufio.NewReader(logStream)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			s.Logger.Error("Error reading log stream", "error", err)
			break
		}

		if err := c.WriteMessage(websocket.TextMessage, line); err != nil {
			s.Logger.Error("Error writing to websocket", "error", err)
			break
		}
	}
}

// Helper method to send errors over the websocket
func (s *PodService) sendLogError(c *websocket.Conn, message string) {
	if err := c.WriteJSON(map[string]string{"error": message}); err != nil {
		s.Logger.Error("Failed to send error message over websocket", "error", err)
	}

	time.Sleep(100 * time.Millisecond) // Give time for the message to be sent

	if err := c.Close(); err != nil {
		s.Logger.Error("Failed to close websocket connection", "error", err)
	}
}
