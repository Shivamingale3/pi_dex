package core

type EventBus struct {
	ch chan Event
}

func NewEventBus(buffer int) *EventBus {
	return &EventBus{ch: make(chan Event, buffer)}
}

func (b *EventBus) Publish(event Event) {
	b.ch <- event
}

func (b *EventBus) Subscribe() <-chan Event {
	return b.ch
}

func (b *EventBus) QSize() int {
	return len(b.ch)
}
