package core

import "time"

type Severity string

const (
	INFO     Severity = "INFO"
	WARN     Severity = "WARN"
	ERROR    Severity = "ERROR"
	CRITICAL Severity = "CRITICAL"
)

type Event struct {
	ID        string
	Hostname  string
	Source    string
	EventType string
	Severity  Severity
	Title     string
	Message   string
	Timestamp time.Time
}