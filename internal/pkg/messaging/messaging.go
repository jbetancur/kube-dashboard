package messaging

import (
	"fmt"
	"log/slog"

	"github.com/jbetancur/dashboard/internal/pkg/grpc"
	messagingtypes "github.com/jbetancur/dashboard/internal/pkg/messaging/types"
)

// ProviderType represents the type of messaging provider
type ProviderType string

const (
	// GRPCProvider represents gRPC messaging
	GRPCProvider ProviderType = "grpc"

	// KafkaProvider represents Kafka messaging
	KafkaProvider ProviderType = "kafka"
)

// Config stores configuration for messaging providers
type Config struct {
	Type          ProviderType
	ServerAddress string // Address for server to listen on
	ClientAddress string // Address for client to connect to
}

// NewClient creates a new messaging client based on the provider type
func NewClient(config Config, logger *slog.Logger) (messagingtypes.MessageQueue, error) {
	switch config.Type {
	case GRPCProvider:
		return grpc.NewAdapter(config.ServerAddress, config.ClientAddress, logger)
	case KafkaProvider:
		// Future implementation
		logger.Warn("Kafka provider not yet implemented, using gRPC")
		return grpc.NewAdapter(config.ServerAddress, config.ClientAddress, logger)
	default:
		return nil, fmt.Errorf("unknown provider type: %s", config.Type)
	}
}
