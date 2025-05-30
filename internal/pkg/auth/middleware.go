package auth

import (
	"fmt"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// SuperUser is a predefined admin user for testing
var SuperUser = UserAttributes{
	Username: "admin-user",
	UID:      "admin-uid",
	Groups:   []string{"system:masters"},
}

// extractAndValidateToken gets a token from the specified source and validates it
func extractAndValidateToken(tokenString string) (UserAttributes, error) {
	// Check for dev mode first
	if os.Getenv("DEV_MODE") == "true" {
		return SuperUser, nil
	}

	// Validate token string
	if tokenString == "" {
		return UserAttributes{}, fmt.Errorf("missing authentication token")
	}

	// Get secret key (with fallback)
	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		secretKey = "default-jwt-secret"
	}

	// Parse token with explicit validation of signing method
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Ensure we're using the expected signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secretKey), nil
	})

	if err != nil || !token.Valid {
		return UserAttributes{}, fmt.Errorf("invalid token: %v", err)
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return UserAttributes{}, fmt.Errorf("invalid token claims")
	}

	// Create UserAttributes from claims
	username, _ := claims["sub"].(string)
	uid, _ := claims["uid"].(string)

	// Extract groups
	var groups []string
	if groupsInterface, ok := claims["groups"].([]interface{}); ok {
		for _, g := range groupsInterface {
			if group, ok := g.(string); ok {
				groups = append(groups, group)
			}
		}
	}

	// Create user attributes
	user := UserAttributes{
		Username: username,
		UID:      uid,
		Groups:   groups,
	}

	return user, nil
}

// AuthMiddleware extracts user information from JWT tokens in the Authorization header
func AuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip auth for OPTIONS requests (CORS preflight)
		if c.Method() == "OPTIONS" {
			return c.Next()
		}

		// DEV_MODE: Simple bypass for development
		if os.Getenv("DEV_MODE") == "true" {
			c.Locals("user", SuperUser)
			return c.Next()
		}

		// Get token from Authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" || len(authHeader) <= 7 || !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing or invalid token",
			})
		}

		// Get just the token part
		tokenString := authHeader[7:]

		// Use shared function to validate token and get user
		user, err := extractAndValidateToken(tokenString)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authentication failed: " + err.Error(),
			})
		}

		// Store user in context
		c.Locals("user", user)
		return c.Next()
	}
}

// WebSocketAuthMiddleware authenticates WebSocket connections using either query param or header
func WebSocketAuthMiddleware(authorizer Authorizer) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip auth for OPTIONS requests (CORS preflight)
		if c.Method() == "OPTIONS" {
			return c.Next()
		}

		// DEV_MODE: Simple bypass for development
		if os.Getenv("DEV_MODE") == "true" {
			c.Locals("user", SuperUser)
			return c.Next()
		}

		// WebSockets typically send auth token via query parameter
		tokenString := c.Query("token")

		// If not found in query, try Authorization header as fallback
		if tokenString == "" {
			authHeader := c.Get("Authorization")
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				tokenString = authHeader[7:]
			}
		}

		// No token found
		if tokenString == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authentication token required",
			})
		}

		// Use shared function to validate token and get user
		user, err := extractAndValidateToken(tokenString)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authentication failed: " + err.Error(),
			})
		}

		// Store user in context
		c.Locals("user", user)

		// Check if user has permission to view pod logs (only if authorizer is provided)
		if authorizer != nil {
			clusterID := c.Params("clusterID")
			namespace := c.Params("namespaceID")
			podName := c.Params("podID")

			allowed, err := authorizer.CanAccess(
				c.Context(),
				clusterID,
				user,
				"pods/log",
				namespace,
				podName,
				"get",
			)

			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to verify permissions",
				})
			}

			if !allowed {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": "You don't have permission to view logs for this pod",
				})
			}
		}

		// Authentication and authorization successful, proceed to WebSocket handler
		return c.Next()
	}
}
