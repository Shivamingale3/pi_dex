package parser

import (
	"regexp"
	"strings"

	"github.com/leadows/pi_dex/internal/config"
	"github.com/leadows/pi_dex/internal/core"
)

type compiledCustomService struct {
	name   string
	events []compiledCustomEvent
}

type compiledCustomEvent struct {
	name     string
	pattern  *regexp.Regexp
	severity string
	title    string
	message  string
}

func MakeCustomServiceParser(services []config.CustomServiceConfig) func(map[string]any) *core.Event {
	compiled := make([]compiledCustomService, 0, len(services))
	for _, svc := range services {
		cs := compiledCustomService{name: svc.Name}
		for _, evt := range svc.Events {
			re, err := regexp.Compile(evt.Pattern)
			if err != nil {
				continue
			}
			cs.events = append(cs.events, compiledCustomEvent{
				name:     evt.Name,
				pattern:  re,
				severity: evt.Severity,
				title:    evt.Title,
				message:  evt.Message,
			})
		}
		if len(cs.events) > 0 {
			compiled = append(compiled, cs)
		}
	}

	return func(entry map[string]any) *core.Event {
		ident, _ := entry["SYSLOG_IDENTIFIER"].(string)
		if ident == "" {
			unit, _ := entry["_SYSTEMD_UNIT"].(string)
			ident = strings.TrimSuffix(unit, ".service")
		}
		if ident == "" {
			return nil
		}

		for _, svc := range compiled {
			if svc.name != ident {
				continue
			}
			msg := entryMessage(entry)
			if msg == "" {
				return nil
			}
			for _, evt := range svc.events {
				if evt.pattern.MatchString(msg) {
					return &core.Event{
						Source:    svc.name,
						EventType: evt.name,
						Severity:  evt.severity,
						Title:     evt.title,
						Message:   evt.message,
						Timestamp: entryTimestamp(entry),
					}
				}
			}
			return nil
		}
		return nil
	}
}
