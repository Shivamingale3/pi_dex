package poller

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/leadows/pi_dex/internal/core"
)

type TemperaturePoller struct {
	BasePoller
}

func NewTemperaturePoller(bus *core.EventBus, interval int, warn, critical float64) *TemperaturePoller {
	return &TemperaturePoller{
		BasePoller: New(bus, interval, warn, critical, core.SourceTemperature),
	}
}

func (p *TemperaturePoller) Run(ctx context.Context) {
	for {
		event := p.checkTemp()
		if event != nil {
			p.Bus.Publish(*event)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(p.Interval) * time.Second):
		}
	}
}

func (p *TemperaturePoller) checkTemp() *core.Event {
	value, err := p.readValue()
	if err != nil {
		return nil
	}

	switch p.state {
	case stateOK:
		if value >= p.Warn {
			p.state = stateWarn
			return p.makeTempEvent(core.SeverityWarn, value)
		}
	case stateWarn:
		if value >= p.Critical {
			p.state = stateCritical
			return p.makeTempEvent(core.SeverityCritical, value)
		}
		if value < p.Warn {
			p.state = stateOK
			return p.makeTempEvent(core.SeverityRecovered, value)
		}
	case stateCritical:
		if value < p.Warn {
			p.state = stateOK
			return p.makeTempEvent(core.SeverityRecovered, value)
		}
	}

	return nil
}

func (p *TemperaturePoller) readValue() (float64, error) {
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return 0, err
	}
	millidegrees, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, err
	}
	return float64(millidegrees) / 1000.0, nil
}

func (p *TemperaturePoller) makeTempEvent(severity string, value float64) *core.Event {
	return &core.Event{
		Source:    p.source,
		EventType: fmt.Sprintf("%s_%s", p.source, severity),
		Severity:  severity,
		Title:     fmt.Sprintf("Temperature %s", severity),
		Message:   fmt.Sprintf("CPU temperature at %.1f°C (warn=%.0f°C, crit=%.0f°C)", value, p.Warn, p.Critical),
		Timestamp: time.Now(),
	}
}
