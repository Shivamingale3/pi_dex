package parser

import (
	"regexp"

	"github.com/leadows/pi_dex/internal/core"
)

var sudoRE = regexp.MustCompile(`^\s*(\S+)\s*:.*COMMAND=(.*)`)

func ParseSudo(entry map[string]any) *core.Event {
	comm, _ := entry["_COMM"].(string)
	ident, _ := entry["SYSLOG_IDENTIFIER"].(string)
	if comm != "sudo" && ident != "sudo" {
		return nil
	}

	msg := entryMessage(entry)
	if msg == "" {
		return nil
	}

	m := sudoRE.FindStringSubmatch(msg)
	if m == nil {
		return nil
	}

	return &core.Event{
		Source:    core.SourcSudo,
		EventType: core.EventSudoUsed,
		Severity:  core.SeverityInfo,
		Title:     "Sudo Used",
		Message:   m[1] + " ran sudo " + m[2],
		Timestamp: entryTimestamp(entry),
	}
}
