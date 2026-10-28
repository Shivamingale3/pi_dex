package parser

import (
	"testing"

	"github.com/Shivamingale3/pi_dex/internal/core"
)

func TestIsWatched(t *testing.T) {
	cases := []struct {
		service  string
		patterns []string
		want     bool
	}{
		{"cloudflared.service", []string{"cloudflared"}, true},
		{"cloudflared.service", []string{"cloudflared.service"}, true},
		{"redis-cache.service", []string{"redis-*"}, true},
		{"redis-cache.service", []string{"redis-*.service"}, true},
		{"redis.service", []string{"redis-*"}, false},
		{"nginx.service", []string{"cloudflared", "docker"}, false},
		{"cloudflared-tunnel.service", []string{"cloudflared"}, false},
		{"anything.service", nil, true},
		{"docker.service", []string{" docker "}, true},
	}
	for _, c := range cases {
		if got := isWatched(c.service, c.patterns); got != c.want {
			t.Errorf("isWatched(%q, %q) = %v, want %v", c.service, c.patterns, got, c.want)
		}
	}
}

func systemdEntry(msg, unit string) map[string]any {
	e := map[string]any{
		"SYSLOG_IDENTIFIER": "systemd",
		"_SYSTEMD_UNIT":     "init.scope",
		"MESSAGE":           msg,
	}
	if unit != "" {
		e["UNIT"] = unit
	}
	return e
}

func TestSystemdParser(t *testing.T) {
	cases := []struct {
		name     string
		msg      string
		unit     string
		wantType string
		wantMsg  string
	}{
		{"started with unit in message", "Started cloudflared.service - Cloudflare Tunnel.", "cloudflared.service",
			core.EventServiceStarted, "cloudflared.service started"},
		{"stopped", "Stopped cloudflared.service - Cloudflare Tunnel.", "cloudflared.service",
			core.EventServiceStopped, "cloudflared.service stopped"},
		{"description only, mapped via UNIT", "Started Cloudflare Tunnel.", "cloudflared.service",
			core.EventServiceStarted, "cloudflared.service started"},
		{"failed with result", "cloudflared.service: Failed with result 'exit-code'.", "cloudflared.service",
			core.EventServiceFailed, "cloudflared.service failed (result: exit-code)"},
		{"dependency failed uses UNIT", "Dependency failed for Cloudflare Tunnel.", "cloudflared.service",
			core.EventServiceFailed, "cloudflared.service failed (dependency failed)"},
		{"failed to start", "Failed to start cloudflared.service - Cloudflare Tunnel.", "cloudflared.service",
			core.EventServiceFailed, "cloudflared.service failed to start"},
		{"no UNIT field", "Started cloudflared.service - Cloudflare Tunnel.", "",
			core.EventServiceStarted, "cloudflared.service started"},
		{"unwatched", "Started nginx.service - nginx.", "nginx.service", "", ""},
		{"description without UNIT is ignored", "Started Cloudflare Tunnel.", "", "", ""},
		{"non-service unit", "Started Session 4 of User pi.", "session-4.scope", "", ""},
		{"target", "Stopped target network-online.target.", "network-online.target", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			parse := MakeSystemdParser([]string{"cloudflared", "redis-*"})
			ev := parse(systemdEntry(c.msg, c.unit))
			if c.wantType == "" {
				if ev != nil {
					t.Fatalf("expected no event, got %+v", ev)
				}
				return
			}
			if ev == nil {
				t.Fatalf("expected %s event, got nil", c.wantType)
			}
			if ev.EventType != c.wantType || ev.Message != c.wantMsg {
				t.Errorf("got %s %q, want %s %q", ev.EventType, ev.Message, c.wantType, c.wantMsg)
			}
		})
	}
}

func TestSystemdParserReportsFailureOnce(t *testing.T) {
	parse := MakeSystemdParser([]string{"redis-*"})
	first := parse(systemdEntry("redis-cache.service: Failed with result 'exit-code'.", "redis-cache.service"))
	if first == nil || first.EventType != core.EventServiceFailed {
		t.Fatalf("expected SERVICE_FAILED, got %+v", first)
	}
	if second := parse(systemdEntry("Failed to start redis-cache.service - Redis.", "redis-cache.service")); second != nil {
		t.Errorf("follow-up 'Failed to start' should be suppressed, got %+v", second)
	}
	if other := parse(systemdEntry("Failed to start redis-queue.service - Redis.", "redis-queue.service")); other == nil {
		t.Error("failure of a different unit must still be reported")
	}
}
