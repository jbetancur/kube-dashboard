package services

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/jbetancur/dashboard/internal/pkg/resources/pods"
)

type PodService struct {
	BaseService
	provider *pods.MultiClusterPodProvider
}

func NewPodService(provider *pods.MultiClusterPodProvider, logger *slog.Logger) *PodService {
	return &PodService{
		BaseService: BaseService{Logger: logger},
		provider:    provider,
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

	s.Logger.Info("Listing pods",
		"clusterID", clusterID,
		"namespaceID", namespaceID)

	pods, err := s.provider.ListPods(c.Context(), clusterID, namespaceID)
	if err != nil {
		return s.Error(c, fiber.StatusInternalServerError, "failed to list pods: %v", err)
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

	s.Logger.Info("Getting pod",
		"clusterID", clusterID,
		"namespaceID", namespaceID,
		"podID", podID)

	pod, err := s.provider.GetPod(c.Context(), clusterID, namespaceID, podID)
	if err != nil {
		return s.Error(c, fiber.StatusInternalServerError, "failed to get pod: %v", err)
	}

	if pod == nil {
		return s.NotFound(c, "Pod", podID)
	}

	return c.JSON(pod)
}

func (s *PodService) StreamPodLogs(c *websocket.Conn) {
	// Create a context with timeout for initial log fetching
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Extract parameters
	clusterID := c.Params("clusterID")
	namespaceID := c.Params("namespaceID")
	podID := c.Params("podID")
	containerName := c.Params("containerName")

	// Parameter validation
	if clusterID == "" {
		s.writeErrorAndClose(c, "missing cluster ID")
		return
	}

	if namespaceID == "" {
		s.writeErrorAndClose(c, "missing namespace ID")
		return
	}

	if podID == "" {
		s.writeErrorAndClose(c, "missing pod ID")
		return
	}

	s.Logger.Info("Streaming pod logs",
		"clusterID", clusterID,
		"namespaceID", namespaceID,
		"podID", podID,
		"container", containerName)

	// Default to 100 lines
	tailLines := int64(100)

	// Get the log stream using the provider (not s.GetPodLogs)
	logStream, err := s.provider.GetPodLogs(ctx, clusterID, namespaceID, podID, containerName, tailLines)
	if err != nil {
		s.Logger.Error("Failed to get pod logs",
			"error", err,
			"clusterID", clusterID,
			"namespaceID", namespaceID,
			"podID", podID)
		s.writeErrorAndClose(c, fmt.Sprintf("failed to get logs: %v", err))
		return
	}
	defer logStream.Close()

	// Stream logs to the WebSocket client
	s.streamLogsToClient(c, logStream)
}

// Helper methods to improve code organization
func (s *PodService) writeErrorAndClose(c *websocket.Conn, message string) {
	s.Logger.Error(message)
	c.WriteJSON(fiber.Map{"error": message})
	c.Close()
}

func (s *PodService) streamLogsToClient(c *websocket.Conn, logStream io.ReadCloser) {
	buffer := make([]byte, 4096) // Increased buffer size for efficiency
	for {
		n, err := logStream.Read(buffer)
		if err != nil {
			if err != io.EOF {
				s.Logger.Error("Error reading logs", "error", err)
			} else {
				s.Logger.Info("Log stream ended")
			}
			break
		}

		if n > 0 {
			err = c.WriteMessage(websocket.TextMessage, buffer[:n])
			if err != nil {
				s.Logger.Error("Error writing to websocket", "error", err)
				break
			}
		}
	}

	c.Close()
}
