package core

import "time"

type Event struct {
	Source    string
	EventType string
	Severity  string
	Title     string
	Message   string
	Timestamp time.Time
}

func (e Event) DedupKey() string {
	return e.Source + "|" + e.EventType + "|" + e.Title + "|" + e.Message
}
