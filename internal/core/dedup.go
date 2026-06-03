package core

import "sync"

type DedupManager struct {
	mu       sync.Mutex
	lastKeys map[string]string
}

func NewDedupManager() *DedupManager {
	return &DedupManager{lastKeys: make(map[string]string)}
}

func (m *DedupManager) IsNew(event Event) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := event.DedupKey()
	last, ok := m.lastKeys[event.Source]
	if !ok {
		return true
	}
	return last != key
}

func (m *DedupManager) Record(event Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastKeys[event.Source] = event.DedupKey()
}

func (m *DedupManager) Reset(source string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.lastKeys, source)
}
