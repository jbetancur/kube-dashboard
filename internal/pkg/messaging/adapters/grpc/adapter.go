package grpc

import (
	"context"
	"log/slog"

	messagingtypes "github.com/jbetancur/dashboard/internal/pkg/messaging/types"
)

// GRPCAdapter implements MessageQueue using gRPC
type GRPCAdapter struct {
	client        *GRPCClient
	server        *GRPCServer
	serverAddress string
	clientAddress string
	logger        *slog.Logger
}

// NewAdapter creates a new adapter that implements MessageQueue
func NewAdapter(serverAddress, clientAddress string, logger *slog.Logger) (messagingtypes.MessageQueue, error) {
	return &GRPCAdapter{
		client:        NewGRPCClient(),
		server:        NewGRPCServer(),
		serverAddress: serverAddress,
		clientAddress: clientAddress,
		logger:        logger,
	}, nil
}

// Connect establishes the connection to the gRPC server for publishing
func (a *GRPCAdapter) Connect(ctx context.Context) error {
	a.logger.Info("Connecting to gRPC server", "address", a.clientAddress)
	return a.client.Connect(ctx, a.clientAddress)
}

// Start begins listening for events
func (a *GRPCAdapter) Start(ctx context.Context) error {
	a.logger.Info("Starting gRPC server", "address", a.serverAddress)
	return a.server.Start(ctx, a.serverAddress)
}

// Publish sends an event to a topic
func (a *GRPCAdapter) Publish(topic string, message []byte) error {
	return a.client.Publish(topic, message)
}

// Subscribe registers a handler for a topic
func (a *GRPCAdapter) Subscribe(topic string, handler func([]byte) error) {
	a.server.Subscribe(topic, handler)
}

// Close closes the gRPC client connection
func (a *GRPCAdapter) Close() error {
	return a.client.Close()
}

// Stop stops the gRPC server
func (a *GRPCAdapter) Stop() error {
	a.server.Stop()
	return nil
}
