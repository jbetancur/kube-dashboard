package messagingtypes

import "context"

// Publisher defines an interface for publishing events
type Publisher interface {
	// Publish sends an event to a topic
	Publish(topic string, message []byte) error
}

// Subscriber defines an interface for subscribing to events
type Subscriber interface {
	// Subscribe registers a handler for a topic
	Subscribe(topic string, handler func([]byte) error)
}

// MessageQueue combines Publisher and Subscriber capabilities
type MessageQueue interface {
	Publisher
	Subscriber

	// Connect establishes a connection for publishing
	Connect(ctx context.Context) error

	// Start begins listening for events (for subscribers)
	Start(ctx context.Context) error

	// Close closes all connections
	Close() error

	// Stop stops the subscriber
	Stop() error
}
