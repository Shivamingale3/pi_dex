package core

import (
	"sync"
	"time"
)

var defaultCooldowns = map[string]float64{
	EventSSHLogin:     CooldownSSHLogin,
	EventSSHLogout:    CooldownSSHLogout,
	EventSSHBruteforce: CooldownSSHBruteforce,
	EventSudoUsed:     CooldownSudoUsed,
	EventCPUHigh:      CooldownCPUHigh,
	EventCPURecovered: CooldownCPURecovered,
	EventTempWarn:     CooldownTempWarn,
	EventTempCritical: CooldownTempCritical,
	EventDiskWarn:     CooldownDiskWarn,
	EventDiskCritical: CooldownDiskCritical,
	EventRAMHigh:      CooldownRAMHigh,
}

type CooldownManager struct {
	mu         sync.Mutex
	cooledUntil map[string]time.Time
	durations   map[string]time.Duration
}

func NewCooldownManager(overrides map[string]float64) *CooldownManager {
	durs := make(map[string]time.Duration, len(defaultCooldowns))
	for k, v := range defaultCooldowns {
		durs[k] = time.Duration(v) * time.Second
	}
	for k, v := range overrides {
		durs[k] = time.Duration(v) * time.Second
	}
	return &CooldownManager{
		cooledUntil: make(map[string]time.Time),
		durations:   durs,
	}
}

func (m *CooldownManager) IsAllowed(event Event) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	deadline, ok := m.cooledUntil[event.EventType]
	if !ok {
		return true
	}
	return time.Now().After(deadline)
}

func (m *CooldownManager) Record(event Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	dur, ok := m.durations[event.EventType]
	if !ok || dur == 0 {
		return
	}
	m.cooledUntil[event.EventType] = time.Now().Add(dur)
}

func (m *CooldownManager) UpdateDuration(eventType string, seconds float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	dur := time.Duration(seconds) * time.Second
	m.durations[eventType] = dur
	if seconds == 0 {
		delete(m.cooledUntil, eventType)
	}
}
