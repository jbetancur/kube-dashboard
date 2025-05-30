package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jbetancur/dashboard/internal/pkg/cluster"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// K8sAuthorizer handles authorization using Kubernetes RBAC
type K8sAuthorizer struct {
	clusterManager *cluster.Manager
	logger         *slog.Logger
	cache          map[string]CachedDecision
	cacheMu        sync.RWMutex
	cacheDuration  time.Duration
}

// NewK8sAuthorizer creates a new authorizer that uses the cluster manager
func NewK8sAuthorizer(clusterManager *cluster.Manager, logger *slog.Logger) *K8sAuthorizer {
	return &K8sAuthorizer{
		clusterManager: clusterManager,
		logger:         logger,
		cache:          make(map[string]CachedDecision),
		cacheDuration:  30 * time.Second,
	}
}

// GetName returns the name of this authorizer implementation
func (a *K8sAuthorizer) GetName() string {
	return "KubernetesRBAC"
}

// CanAccess checks if a user has permission to perform an action
func (a *K8sAuthorizer) CanAccess(ctx context.Context, clusterID string, user UserAttributes,
	resource, namespace, name, verb string) (bool, error) {
	// Generate cache key
	cacheKey := fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s",
		user.Username, clusterID, resource, namespace, name, verb,
		strings.Join(user.Groups, ","))

	// Check cache
	a.cacheMu.RLock()
	if decision, exists := a.cache[cacheKey]; exists {
		if time.Since(decision.Timestamp) < a.cacheDuration {
			a.cacheMu.RUnlock()
			return decision.Allowed, nil
		}
	}
	a.cacheMu.RUnlock()

	// Get cluster connection
	conn, err := a.clusterManager.GetCluster(clusterID)
	if err != nil {
		return false, fmt.Errorf("failed to get cluster connection: %w", err)
	}

	if conn == nil || !conn.IsConnected() {
		return false, fmt.Errorf("cluster %s not connected", clusterID)
	}

	// Create SubjectAccessReview
	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: namespace,
				Verb:      verb,
				Resource:  resource,
				Name:      name,
			},
			User:   user.Username,
			UID:    user.UID,
			Groups: user.Groups,
			Extra:  convertExtra(user.Extra),
		},
	}

	// Submit to Kubernetes API
	result, err := conn.Client.AuthorizationV1().SubjectAccessReviews().Create(
		ctx, sar, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("authorization check failed: %w", err)
	}

	// Cache result
	a.cacheMu.Lock()
	a.cache[cacheKey] = CachedDecision{
		Allowed:   result.Status.Allowed,
		Timestamp: time.Now(),
	}
	a.cacheMu.Unlock()

	// Log the result
	a.logger.Debug("Access check",
		"user", user.Username,
		"resource", resource,
		"namespace", namespace,
		"name", name,
		"verb", verb,
		"cluster", clusterID,
		"allowed", result.Status.Allowed)

	return result.Status.Allowed, nil
}

// Helper function to convert extra map
func convertExtra(extra map[string][]string) map[string]authorizationv1.ExtraValue {
	if extra == nil {
		return nil
	}
	converted := make(map[string]authorizationv1.ExtraValue)
	for k, v := range extra {
		converted[k] = v
	}
	return converted
}
