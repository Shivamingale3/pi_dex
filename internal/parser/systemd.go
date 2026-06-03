package parser

import (
	"path"
	"regexp"

	"github.com/leadows/pi_dex/internal/core"
)

var (
	svcStartedRE   = regexp.MustCompile(`Started (.+\.service)`)
	svcStoppedRE   = regexp.MustCompile(`Stopped (.+\.service)`)
	svcFailedRE    = regexp.MustCompile(`Failed to start (.+\.service)`)
	svcRestartedRE = regexp.MustCompile(`Restarted (.+\.service)`)
)

func MakeSystemdParser(watchPatterns []string) func(map[string]any) *core.Event {
	return func(entry map[string]any) *core.Event {
		ident, _ := entry["SYSLOG_IDENTIFIER"].(string)
		unit, _ := entry["_SYSTEMD_UNIT"].(string)

		if ident != "systemd" && !isServiceUnit(unit) {
			return nil
		}

		msg := entryMessage(entry)
		if msg == "" {
			return nil
		}

		svc := extractService(msg)
		if svc == "" {
			return nil
		}

		if !isWatched(svc, watchPatterns) {
			return nil
		}

		switch {
		case svcStartedRE.MatchString(msg):
			return &core.Event{
				Source:    core.SourceSystemd,
				EventType: core.EventServiceStarted,
				Severity:  core.SeverityInfo,
				Title:     "Service Started",
				Message:   svc + " started",
				Timestamp: entryTimestamp(entry),
			}
		case svcStoppedRE.MatchString(msg):
			return &core.Event{
				Source:    core.SourceSystemd,
				EventType: core.EventServiceStopped,
				Severity:  core.SeverityWarn,
				Title:     "Service Stopped",
				Message:   svc + " stopped",
				Timestamp: entryTimestamp(entry),
			}
		case svcFailedRE.MatchString(msg):
			return &core.Event{
				Source:    core.SourceSystemd,
				EventType: core.EventServiceFailed,
				Severity:  core.SeverityCritical,
				Title:     "Service Failed",
				Message:   svc + " failed to start",
				Timestamp: entryTimestamp(entry),
			}
		case svcRestartedRE.MatchString(msg):
			return &core.Event{
				Source:    core.SourceSystemd,
				EventType: core.EventServiceRestarted,
				Severity:  core.SeverityInfo,
				Title:     "Service Restarted",
				Message:   svc + " was restarted",
				Timestamp: entryTimestamp(entry),
			}
		}

		return nil
	}
}

func isServiceUnit(unit string) bool {
	return path.Ext(unit) == ".service"
}

func extractService(msg string) string {
	for _, re := range []*regexp.Regexp{svcStartedRE, svcStoppedRE, svcFailedRE, svcRestartedRE} {
		if m := re.FindStringSubmatch(msg); m != nil {
			return m[1]
		}
	}
	return ""
}

func isWatched(service string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		if matched, _ := path.Match(p, service); matched {
			return true
		}
	}
	return false
}
