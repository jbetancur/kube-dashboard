package services

import (
	"fmt"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/jbetancur/dashboard/internal/pkg/auth"
)

// BaseService provides common functionality for all services
type BaseService struct {
	Logger *slog.Logger
}

// Error returns a standardized error response
func (s *BaseService) Error(c *fiber.Ctx, status int, format string, args ...interface{}) error {
	message := format
	if len(args) > 0 {
		message = fmt.Sprintf(format, args...)
	}

	if s.Logger != nil {
		s.Logger.Error(message)
	}

	return c.Status(status).JSON(fiber.Map{
		"error": message,
	})
}

// NotFound returns a standardized 404 response
func (s *BaseService) NotFound(c *fiber.Ctx, resourceType string, id string) error {
	message := fmt.Sprintf("%s not found with id: %s", resourceType, id)
	if s.Logger != nil {
		s.Logger.Warn(message)
	}
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"error": message,
	})
}

// BadRequest returns a standardized 400 response
func (s *BaseService) BadRequest(c *fiber.Ctx, message string) error {
	if s.Logger != nil {
		s.Logger.Warn("Bad request: " + message)
	}
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error": message,
	})
}

// InternalServerError returns a standardized 500 response
func (s *BaseService) InternalServerError(c *fiber.Ctx, message string, err error) error {
	if s.Logger != nil {
		s.Logger.Error("Internal server error: "+message, "error", err)
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": message,
	})
}

// CheckResourcePermission checks if a user has permission to access a resource
func (s *BaseService) CheckResourcePermission(c *fiber.Ctx, authorizer auth.Authorizer,
	resource, namespace, name, verb string) (auth.UserAttributes, error) {
	// Get user from context
	user, ok := c.Locals("user").(auth.UserAttributes)
	if !ok {
		return user, c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User information not available",
		})
	}

	// Get clusterID from path parameters
	clusterID := c.Params("clusterID")
	if clusterID == "" {
		return user, s.BadRequest(c, "Missing cluster ID")
	}

	// Check permission
	allowed, err := authorizer.CanAccess(c.Context(), clusterID, user, resource, namespace, name, verb)
	if err != nil {
		s.Logger.Error("Failed to check permissions",
			"error", err,
			"resource", resource,
			"namespace", namespace,
			"name", name)
		return user, s.Error(c, fiber.StatusInternalServerError, "Failed to verify permissions")
	}

	if !allowed {
		return user, s.Error(c, fiber.StatusForbidden,
			"You don't have permission to %s this %s", verb, resource)
	}

	return user, nil
}
