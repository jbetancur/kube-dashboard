package auth

import (
	"context"
	"time"
)

// Authorizer defines the interface for performing authorization checks
type Authorizer interface {
	// CanAccess checks if a user has permission to perform an action
	CanAccess(ctx context.Context, clusterID string, user UserAttributes,
		resource, namespace, name, verb string) (bool, error)

	// GetName returns the name of the authorizer implementation
	GetName() string
}

// UserAttributes holds information about the user requesting access
type UserAttributes struct {
	Username string              `json:"username"`
	UID      string              `json:"uid,omitempty"`
	Groups   []string            `json:"groups,omitempty"`
	Extra    map[string][]string `json:"extra,omitempty"`
}

// CachedDecision represents a cached authorization decision
type CachedDecision struct {
	Allowed   bool
	Timestamp time.Time
}
