package grpc

import (
	context "context"
	"fmt"
	"net"
	sync "sync"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCServer handles incoming gRPC requests
type GRPCServer struct {
	server   *grpc.Server
	handlers map[string][]func([]byte) error
	mu       sync.RWMutex
	UnimplementedEventServiceServer
}

// GRPCClient handles outgoing gRPC requests
type GRPCClient struct {
	client EventServiceClient
	conn   *grpc.ClientConn
	mu     sync.RWMutex
}

// NewGRPCServer creates a new GRPCServer instance
func NewGRPCServer() *GRPCServer {
	return &GRPCServer{
		server:   grpc.NewServer(),
		handlers: make(map[string][]func([]byte) error),
	}
}

// NewGRPCClient creates a new GRPCClient instance
func NewGRPCClient() *GRPCClient {
	return &GRPCClient{}
}

// Start initializes and starts the gRPC server
func (s *GRPCServer) Start(ctx context.Context, address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	// Register the server implementation
	RegisterEventServiceServer(s.server, s)

	// Start the server in a goroutine
	go func() {
		if err := s.server.Serve(listener); err != nil {
			fmt.Printf("gRPC server error: %v\n", err)
		}
	}()

	fmt.Printf("gRPC server started on %s\n", address)

	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	return nil
}

// Stop gracefully stops the gRPC server
func (s *GRPCServer) Stop() {
	if s.server != nil {
		s.server.GracefulStop()
		fmt.Println("gRPC server stopped gracefully")
	}
}

// Server methods
func (s *GRPCServer) PublishEvent(ctx context.Context, req *EventRequest) (*EventResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// logger.Info("Received PublishEvent request", "topic", req.Topic)

	handlers, exists := s.handlers[req.Topic]
	if !exists {
		return &EventResponse{Success: false}, nil
	}

	for _, handler := range handlers {
		if err := handler([]byte(req.Payload)); err != nil {
			return &EventResponse{Success: false}, err
		}
	}

	return &EventResponse{Success: true}, nil
}

func (s *GRPCServer) Subscribe(topic string, handler func([]byte) error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.handlers[topic] == nil {
		s.handlers[topic] = make([]func([]byte) error, 0)
	}
	s.handlers[topic] = append(s.handlers[topic], handler)
}

// Client methods
func (c *GRPCClient) Connect(ctx context.Context, address string) error {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	c.conn = conn
	c.client = NewEventServiceClient(conn)

	go func() {
		<-ctx.Done()
		if err := c.Close(); err != nil {
			fmt.Printf("Error closing gRPC client: %v\n", err)
		}
	}()

	return nil
}

func (c *GRPCClient) Publish(topic string, message []byte) error {
	if c.client == nil {
		return fmt.Errorf("client not connected")
	}

	_, err := c.client.PublishEvent(context.Background(), &EventRequest{
		Topic:   topic,
		Payload: string(message),
	})
	return err
}

// Close closes the gRPC client connection
func (c *GRPCClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.client = nil
		return err
	}
	return nil
}
