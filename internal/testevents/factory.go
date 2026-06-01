package testevents

import "PI_DEX/internal/core"

func Get(name string) (core.Event, bool) {

	switch name {

	case "ssh-login":
		return SSHLogin(), true

	case "docker-down":
		return DockerDown(), true

	case "cpu-high":
		return CPUHigh(), true

	default:
		return core.Event{}, false
	}
}
