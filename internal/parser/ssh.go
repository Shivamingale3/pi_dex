package parser

import (
	"regexp"
	"sync"
	"time"

	"github.com/Shivamingale3/pi_dex/internal/core"
)

var (
	acceptedRE = regexp.MustCompile(
		`Accepted (?:publickey|password|keyboard-interactive) for (\S+) from (\S+)`,
	)
	failedRE = regexp.MustCompile(
		`Failed (?:password|publickey) for (\S+) from (\S+)`,
	)
	disconnectedRE = regexp.MustCompile(
		`Disconnected from (?:user\s+)?(\S+) (\S+)`,
	)
)

const (
	bruteforceWindow    = 30
	bruteforceThreshold = 3
	bruteforceMaxIPs    = 1000
)

var bruteforceTracker struct {
	mu   sync.Mutex
	ips  map[string][]time.Time
	keys []string
}

func init() {
	bruteforceTracker.ips = make(map[string][]time.Time)
}

func ParseSSH(entry map[string]any) *core.Event {
	comm, _ := entry["_COMM"].(string)
	ident, _ := entry["SYSLOG_IDENTIFIER"].(string)
	if comm != "sshd" && comm != "sshd-session" && ident != "sshd" && ident != "sshd-session" {
		return nil
	}

	msg := entryMessage(entry)
	if msg == "" {
		return nil
	}

	if m := acceptedRE.FindStringSubmatch(msg); m != nil {
		return &core.Event{
			Source:    core.SourceSSH,
			EventType: core.EventSSHLogin,
			Severity:  core.SeverityInfo,
			Title:     "SSH Login",
			Message:   m[1] + " logged in from " + m[2],
			Timestamp: entryTimestamp(entry),
		}
	}

	if m := disconnectedRE.FindStringSubmatch(msg); m != nil {
		return &core.Event{
			Source:    core.SourceSSH,
			EventType: core.EventSSHLogout,
			Severity:  core.SeverityInfo,
			Title:     "SSH Logout",
			Message:   m[1] + " disconnected from " + m[2],
			Timestamp: entryTimestamp(entry),
		}
	}

	if m := failedRE.FindStringSubmatch(msg); m != nil {
		ip := m[2]
		if isBruteforce(ip) {
			return &core.Event{
				Source:    core.SourceSSH,
				EventType: core.EventSSHBruteforce,
				Severity:  core.SeverityWarn,
				Title:     "SSH Brute Force",
				Message:   "Repeated failed attempts for " + m[1] + " from " + ip,
				Timestamp: entryTimestamp(entry),
			}
		}
	}

	return nil
}

func isBruteforce(ip string) bool {
	bruteforceTracker.mu.Lock()
	defer bruteforceTracker.mu.Unlock()

	attempts, ok := bruteforceTracker.ips[ip]
	if !ok {
		if len(bruteforceTracker.ips) >= bruteforceMaxIPs {
			delete(bruteforceTracker.ips, bruteforceTracker.keys[0])
			bruteforceTracker.keys = bruteforceTracker.keys[1:]
		}
		attempts = nil
		bruteforceTracker.keys = append(bruteforceTracker.keys, ip)
	}

	now := time.Now()
	cutoff := now.Add(-bruteforceWindow * time.Second)

	var active []time.Time
	for _, t := range attempts {
		if t.After(cutoff) {
			active = append(active, t)
		}
	}
	active = append(active, now)
	bruteforceTracker.ips[ip] = active

	return len(active) >= bruteforceThreshold
}
