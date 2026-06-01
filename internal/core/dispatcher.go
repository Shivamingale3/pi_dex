package core

import (
	"log"
	"time"
)

type Dispatcher struct {
	Notifier Notifier
	Dedup    *Deduplicator
	Cooldown *CooldownManager
}

func (d *Dispatcher) Start(
	bus *EventBus,
) {

	for event := range bus.Events {

		if d.Dedup.IsDuplicate(event) {
			log.Println("duplicate event skipped")
			continue
		}

		if !d.Cooldown.Allow(
			event.EventType,
			10*time.Second,
		) {
			log.Println("cooldown active")
			continue
		}

		err := d.Notifier.Send(event)

		if err != nil {
			log.Printf(
				"send failed: %v",
				err,
			)
		}
	}
}