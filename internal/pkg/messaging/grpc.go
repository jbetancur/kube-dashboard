package messaging

import (
	"fmt"
	"sync"

	"google.golang.org/grpc"
)

type GRPCMessageQueue struct {
	server   *grpc.Server
	messages map[string][]func([]byte) error
	mu       sync.RWMutex
}

func NewGRPCMessageQueue() *GRPCMessageQueue {
	return &GRPCMessageQueue{
		server:   grpc.NewServer(),
		messages: make(map[string][]func([]byte) error),
	}
}

func (mq *GRPCMessageQueue) Publish(topic string, message []byte) error {
	mq.mu.RLock()
	defer mq.mu.RUnlock()

	handlers, exists := mq.messages[topic]
	if !exists {
		return fmt.Errorf("no subscribers for topic: %s", topic)
	}

	for _, handler := range handlers {
		if err := handler(message); err != nil {
			return fmt.Errorf("handler error: %w", err)
		}
	}

	return nil
}

func (mq *GRPCMessageQueue) Subscribe(topic string, handler func(message []byte) error) error {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	mq.messages[topic] = append(mq.messages[topic], handler)
	return nil
}

func (mq *GRPCMessageQueue) Close() error {
	mq.server.GracefulStop()
	return nil
}
