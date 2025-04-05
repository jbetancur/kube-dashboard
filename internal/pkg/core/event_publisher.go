package core

import (
	"encoding/json"
	"fmt"

	"github.com/jbetancur/dashboard/internal/pkg/messaging"
)

type EventPublisher struct {
	messageQueue messaging.MessageQueue
}

func NewEventPublisher(mq messaging.MessageQueue) *EventPublisher {
	return &EventPublisher{
		messageQueue: mq,
	}
}

func (ep *EventPublisher) PublishEvent(eventType, topic, clusterID string, object interface{}) {
	event := map[string]interface{}{
		"eventType": eventType,
		"clusterID": clusterID,
		"object":    object,
	}

	data, err := json.Marshal(event)
	if err != nil {
		fmt.Printf("Failed to marshal event: %v\n", err)
		return
	}

	if err := ep.messageQueue.Publish(topic, data); err != nil {
		fmt.Printf("Failed to publish event: %v\n", err)
	}
}
