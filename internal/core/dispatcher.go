package core

import (
	"context"
	"log"
)

type Dispatcher struct {
	bus       *EventBus
	notifier  Notifier
	cooldowns *CooldownManager
	dedup     *DedupManager
}

func NewDispatcher(bus *EventBus, n Notifier, cd *CooldownManager, dd *DedupManager) *Dispatcher {
	return &Dispatcher{
		bus:       bus,
		notifier:  n,
		cooldowns: cd,
		dedup:     dd,
	}
}

// Run processes events until ctx is cancelled. An event being processed when
// ctx is cancelled is finished before Run returns.
func (d *Dispatcher) Run(ctx context.Context) {
	ch := d.bus.Subscribe()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			d.process(event)
		}
	}
}

// DrainPending processes every event still buffered in the bus, then returns.
// Call it after Run has returned, during shutdown.
func (d *Dispatcher) DrainPending() {
	ch := d.bus.Subscribe()
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			d.process(event)
		default:
			return
		}
	}
}

func (d *Dispatcher) process(event Event) {
	if !d.cooldowns.IsAllowed(event) {
		log.Printf("Cooldown active for %s from %s, dropping", event.EventType, event.Source)
		return
	}

	if !d.dedup.IsNew(event) {
		log.Printf("Duplicate %s from %s, dropping", event.EventType, event.Source)
		return
	}

	d.cooldowns.Record(event)
	d.dedup.Record(event)

	if err := d.notifier.Send(event); err != nil {
		log.Printf("Failed to send event %s: %v", event.EventType, err)
	}
}
