package poller

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/leadows/pi_dex/internal/core"
)

type state int

const (
	stateOK state = iota
	stateWarn
	stateCritical
)

type BasePoller struct {
	Bus      *core.EventBus
	Interval int
	Warn     float64
	Critical float64

	state state
	source string
}

func New(bus *core.EventBus, interval int, warn, critical float64, source string) BasePoller {
	return BasePoller{
		Bus:      bus,
		Interval: interval,
		Warn:     warn,
		Critical: critical,
		source:   source,
	}
}

func (p *BasePoller) Run(ctx context.Context, readValue func() (float64, error)) {
	for {
		event := p.check(readValue)
		if event != nil {
			p.Bus.Publish(*event)
			log.Printf("%s: %s", p.source, event.Severity)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(p.Interval) * time.Second):
		}
	}
}

func (p *BasePoller) check(readValue func() (float64, error)) *core.Event {
	value, err := readValue()
	if err != nil {
		return nil
	}

	switch p.state {
	case stateOK:
		if value >= p.Warn {
			p.state = stateWarn
			return makeEvent(p.source, "WARN", value, p.Warn, p.Critical)
		}
	case stateWarn:
		if value >= p.Critical {
			p.state = stateCritical
			return makeEvent(p.source, "CRITICAL", value, p.Warn, p.Critical)
		}
		if value < p.Warn {
			p.state = stateOK
			return makeEvent(p.source, "RECOVERED", value, p.Warn, p.Critical)
		}
	case stateCritical:
		if value < p.Warn {
			p.state = stateOK
			return makeEvent(p.source, "RECOVERED", value, p.Warn, p.Critical)
		}
	}

	return nil
}

func makeEvent(source, severity string, value, warn, critical float64) *core.Event {
	var title string
	switch severity {
	case "WARN":
		title = fmt.Sprintf("%s Warn", source)
	case "CRITICAL":
		title = fmt.Sprintf("%s Critical", source)
	case "RECOVERED":
		title = fmt.Sprintf("%s Recovered", source)
	}

	return &core.Event{
		Source:    source,
		EventType: fmt.Sprintf("%s_%s", source, severity),
		Severity:  severity,
		Title:     title,
		Message:   fmt.Sprintf("%s usage at %.1f%% (warn=%.0f%%, crit=%.0f%%)", source, value, warn, critical),
		Timestamp: time.Now(),
	}
}
