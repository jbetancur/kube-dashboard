package auth

import (
	"fmt"
	"log/slog"

	"github.com/gofiber/fiber/v2"
)

// ResourceInfo defines the resource being accessed
type ResourceInfo struct {
	Resource       string
	ResourceName   string
	Verb           string
	ClusterParam   string
	NamespaceParam string
	NameParam      string
}

// RequirePermission creates a middleware that checks if the user has permission to access a resource
func RequirePermission(authorizer Authorizer, logger *slog.Logger, resourceInfo ResourceInfo) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get user from context
		user, ok := c.Locals("user").(UserAttributes)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "User information not available",
			})
		}

		// Get parameters from URL
		clusterID := c.Params(resourceInfo.ClusterParam)
		if clusterID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Missing %s parameter", resourceInfo.ClusterParam),
			})
		}

		// Get namespace (if applicable)
		var namespace string
		if resourceInfo.NamespaceParam != "" {
			namespace = c.Params(resourceInfo.NamespaceParam)
		}

		// Get resource name (if applicable)
		var name string
		if resourceInfo.NameParam != "" {
			name = c.Params(resourceInfo.NameParam)
		} else if resourceInfo.ResourceName != "" {
			name = resourceInfo.ResourceName
		}

		// Check permission
		allowed, err := authorizer.CanAccess(c.Context(), clusterID, user,
			resourceInfo.Resource, namespace, name, resourceInfo.Verb)
		if err != nil {
			logger.Error("Permission check failed",
				"error", err,
				"resource", resourceInfo.Resource,
				"verb", resourceInfo.Verb)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to verify permissions",
			})
		}

		if !allowed {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": fmt.Sprintf("You don't have permission to %s this %s",
					resourceInfo.Verb, resourceInfo.Resource),
			})
		}

		// Permission granted, continue to the next handler
		return c.Next()
	}
}
