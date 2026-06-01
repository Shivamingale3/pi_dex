package testevents

import (
	"os"
	"time"

	"github.com/google/uuid"

	"PI_DEX/internal/core"
)

func SSHLogin() core.Event {

	hostname, _ := os.Hostname()

	return core.Event{
		ID:        uuid.NewString(),
		Hostname:  hostname,
		Source:    "ssh",
		EventType: "SSH_LOGIN",
		Severity:  core.INFO,
		Title:     "SSH Login",
		Message:   "Test SSH login event",
		Timestamp: time.Now(),
	}
}

func DockerDown() core.Event {

	hostname, _ := os.Hostname()

	return core.Event{
		ID:        uuid.NewString(),
		Hostname:  hostname,
		Source:    "docker",
		EventType: "docker_down",
		Severity:  core.INFO,
		Title:     "Docker Down",
		Message:   "Test Docker Down event",
		Timestamp: time.Now(),
	}
}

func CPUHigh() core.Event {

	host, _ := os.Hostname()

	return core.Event{
		ID: uuid.NewString(),
		Hostname: host,
		Source: "cpu",
		EventType: "CPU_HIGH",
		Severity: core.WARN,
		Title: "CPU Usage High",
		Message: "CPU usage exceeded threshold",
		Timestamp: time.Now(),
	}
}