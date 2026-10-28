package core

import "time"

type Event struct {
	Source    string    `json:"source"`
	EventType string    `json:"event_type"`
	Severity  string    `json:"severity"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Details   string    `json:"details,omitempty"` // preformatted block, e.g. log lines
	Timestamp time.Time `json:"timestamp"`

	// Delayed marks an event that is being delivered from the offline queue.
	Delayed bool `json:"-"`
}

func (e Event) DedupKey() string {
	return e.Source + "|" + e.EventType + "|" + e.Title + "|" + e.Message
}
