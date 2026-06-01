package core

import (
	"sync"
	"time"
)

type CooldownManager struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func NewCooldownManager() *CooldownManager {
	return &CooldownManager{
		last: make(map[string]time.Time),
	}
}

func (c *CooldownManager) Allow(
	eventType string,
	duration time.Duration,
) bool {

	c.mu.Lock()
	defer c.mu.Unlock()

	lastTime, exists := c.last[eventType]

	if !exists {
		c.last[eventType] = time.Now()
		return true
	}

	if time.Since(lastTime) < duration {
		return false
	}

	c.last[eventType] = time.Now()

	return true
}