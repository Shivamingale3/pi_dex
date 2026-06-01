package core

type EventBus struct {
	Events chan Event
}

func NewEventBus(buffer int) *EventBus {
	return &EventBus{
		Events: make(chan Event, buffer),
	}
}