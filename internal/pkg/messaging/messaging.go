package messaging

type Publisher interface {
	Publish(topic string, message []byte) error
}

type Subscriber interface {
	AddHandler(topic string, handler func([]byte) error)
}

type MessageQueue interface {
	Publisher
	Subscriber
}
