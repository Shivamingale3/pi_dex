package core

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

type Deduplicator struct {
	mu   sync.Mutex
	last map[string]string
}

func NewDeduplicator() *Deduplicator {
	return &Deduplicator{
		last: make(map[string]string),
	}
}

func (d *Deduplicator) IsDuplicate(event Event) bool {

	d.mu.Lock()
	defer d.mu.Unlock()

	hashBytes := sha256.Sum256(
		[]byte(
			event.Source +
				event.EventType +
				event.Message,
		),
	)

	hash := hex.EncodeToString(hashBytes[:])

	lastHash, exists := d.last[event.Source]

	if exists && lastHash == hash {
		return true
	}

	d.last[event.Source] = hash

	return false
}