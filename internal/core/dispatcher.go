package core

import (
	"context"
	"log"
	"sync"
)

type Dispatcher struct {
	bus       *EventBus
	notifier  Notifier
	cooldowns *CooldownManager
	dedup     *DedupManager
	inflight  sync.WaitGroup
}

func NewDispatcher(bus *EventBus, n Notifier, cd *CooldownManager, dd *DedupManager) *Dispatcher {
	return &Dispatcher{
		bus:       bus,
		notifier:  n,
		cooldowns: cd,
		dedup:     dd,
	}
}

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
			d.inflight.Add(1)
			d.process(event)
			d.inflight.Done()
		}
	}
}

func (d *Dispatcher) Drain() {
	d.inflight.Wait()
}

func (d *Dispatcher) process(event Event) {
	if !d.cooldowns.IsAllowed(event) {
		log.Printf("Cooldown active for %s, dropping", event.EventType)
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
