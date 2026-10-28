package parser

import (
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Shivamingale3/pi_dex/internal/core"
)

var (
	svcStartedRE    = regexp.MustCompile(`^Started (\S+\.service)`)
	svcStoppedRE    = regexp.MustCompile(`^Stopped (\S+\.service)`)
	svcFailedRE     = regexp.MustCompile(`^Failed to start (\S+\.service)`)
	svcRestartedRE  = regexp.MustCompile(`^Restarted (\S+\.service)`)
	svcResultRE     = regexp.MustCompile(`^(\S+\.service): Failed with result '([^']+)'`)
	svcDependencyRE = regexp.MustCompile(`^Dependency failed for (.+?)\.?$`)

	// Older systemd logs "Started <Description>." without the unit name. Those
	// messages match these looser patterns and rely on the UNIT journal field.
	descStartedRE   = regexp.MustCompile(`^Started (.+)`)
	descStoppedRE   = regexp.MustCompile(`^Stopped (.+)`)
	descFailedRE    = regexp.MustCompile(`^Failed to start (.+)`)
	descRestartedRE = regexp.MustCompile(`^Restarted (.+)`)
)

// A unit that fails logs "<unit>: Failed with result ..." and then, if it was
// starting, "Failed to start <unit>". Only the first one is reported.
const failureDedupWindow = 10 * time.Second

func MakeSystemdParser(watchPatterns []string) func(map[string]any) *core.Event {
	var mu sync.Mutex
	lastFailure := make(map[string]time.Time)

	recentlyFailed := func(svc string, now time.Time) bool {
		mu.Lock()
		defer mu.Unlock()
		t, ok := lastFailure[svc]
		return ok && now.Sub(t) < failureDedupWindow
	}
	markFailed := func(svc string, now time.Time) {
		mu.Lock()
		defer mu.Unlock()
		for k, t := range lastFailure {
			if now.Sub(t) >= failureDedupWindow {
				delete(lastFailure, k)
			}
		}
		lastFailure[svc] = now
	}

	return func(entry map[string]any) *core.Event {
		ident, _ := entry["SYSLOG_IDENTIFIER"].(string)
		unit, _ := entry["_SYSTEMD_UNIT"].(string)

		if ident != "systemd" && !isServiceUnit(unit) {
			return nil
		}

		msg := strings.TrimSpace(entryMessage(entry))
		if msg == "" {
			return nil
		}

		// systemd attaches the unit a message is about as UNIT (system
		// manager) or USER_UNIT (user manager). Prefer it over the message,
		// which may only contain the unit's Description.
		subject, _ := entry["UNIT"].(string)
		if subject == "" {
			subject, _ = entry["USER_UNIT"].(string)
		}
		if subject != "" && !isServiceUnit(subject) {
			return nil
		}

		kind, svc, detail := classify(msg)
		if kind == "" {
			return nil
		}
		if subject != "" {
			svc = subject
		} else if !isServiceUnit(svc) {
			return nil
		}

		if !isWatched(svc, watchPatterns) {
			return nil
		}

		ts := entryTimestamp(entry)
		now := time.Now()

		switch kind {
		case core.EventServiceStarted:
			return &core.Event{
				Source:    core.SourceSystemd,
				EventType: core.EventServiceStarted,
				Severity:  core.SeverityInfo,
				Title:     "Service Started",
				Message:   svc + " started",
				Timestamp: ts,
			}
		case core.EventServiceStopped:
			return &core.Event{
				Source:    core.SourceSystemd,
				EventType: core.EventServiceStopped,
				Severity:  core.SeverityWarn,
				Title:     "Service Stopped",
				Message:   svc + " stopped",
				Timestamp: ts,
			}
		case core.EventServiceRestarted:
			return &core.Event{
				Source:    core.SourceSystemd,
				EventType: core.EventServiceRestarted,
				Severity:  core.SeverityInfo,
				Title:     "Service Restarted",
				Message:   svc + " was restarted",
				Timestamp: ts,
			}
		case core.EventServiceFailed:
			if detail == "" && recentlyFailed(svc, now) {
				return nil
			}
			markFailed(svc, now)
			message := svc + " failed to start"
			if detail != "" {
				message = svc + " failed (" + detail + ")"
			}
			return &core.Event{
				Source:    core.SourceSystemd,
				EventType: core.EventServiceFailed,
				Severity:  core.SeverityCritical,
				Title:     "Service Failed",
				Message:   message,
				Timestamp: ts,
			}
		}

		return nil
	}
}

// classify returns the event type, the service named in the message (a unit
// name, or a Description when systemd did not log the unit) and extra detail.
func classify(msg string) (kind, svc, detail string) {
	if m := svcResultRE.FindStringSubmatch(msg); m != nil {
		return core.EventServiceFailed, m[1], "result: " + m[2]
	}
	if m := svcDependencyRE.FindStringSubmatch(msg); m != nil {
		return core.EventServiceFailed, m[1], "dependency failed"
	}

	patterns := []struct {
		kind         string
		unitRE, desc *regexp.Regexp
	}{
		{core.EventServiceStarted, svcStartedRE, descStartedRE},
		{core.EventServiceStopped, svcStoppedRE, descStoppedRE},
		{core.EventServiceFailed, svcFailedRE, descFailedRE},
		{core.EventServiceRestarted, svcRestartedRE, descRestartedRE},
	}
	for _, p := range patterns {
		if m := p.unitRE.FindStringSubmatch(msg); m != nil {
			return p.kind, m[1], ""
		}
	}
	for _, p := range patterns {
		if m := p.desc.FindStringSubmatch(msg); m != nil {
			return p.kind, m[1], ""
		}
	}
	return "", "", ""
}

func isServiceUnit(unit string) bool {
	return path.Ext(unit) == ".service"
}

// isWatched reports whether service (e.g. "cloudflared.service") matches one
// of the watch patterns. A pattern without the ".service" suffix
// ("cloudflared", "redis-*") is matched against the name with the suffix
// stripped. An empty pattern list watches everything.
func isWatched(service string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	bare := strings.TrimSuffix(service, ".service")
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		target := bare
		if strings.HasSuffix(p, ".service") {
			target = service
		}
		if matched, _ := path.Match(p, target); matched {
			return true
		}
	}
	return false
}
