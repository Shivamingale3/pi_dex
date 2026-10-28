package core

import "testing"

func TestCooldownIsPerSource(t *testing.T) {
	m := NewCooldownManager(map[string]float64{EventHealthDown: 600})
	a := Event{Source: "health:a", EventType: EventHealthDown}
	b := Event{Source: "health:b", EventType: EventHealthDown}

	m.Record(a)
	if m.IsAllowed(a) {
		t.Error("a should be cooling down")
	}
	if !m.IsAllowed(b) {
		t.Error("b must not be suppressed by a's cooldown")
	}

	m.UpdateDuration(EventHealthDown, 0)
	if !m.IsAllowed(a) {
		t.Error("cooldown should be cleared when duration set to 0")
	}
}

func TestDedupIsPerSource(t *testing.T) {
	d := NewDedupManager()
	e := Event{Source: "health:a", EventType: EventHealthDown, Message: "down"}
	d.Record(e)
	if d.IsNew(e) {
		t.Error("identical consecutive event should be a duplicate")
	}
	other := e
	other.Source = "health:b"
	if !d.IsNew(other) {
		t.Error("same event from another source is new")
	}
}
