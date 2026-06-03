package poller

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/leadows/pi_dex/internal/core"
)

type DiskPoller struct {
	BasePoller
	mount string
}

func NewDiskPoller(bus *core.EventBus, interval int, warn, critical float64, mount string) *DiskPoller {
	return &DiskPoller{
		BasePoller: New(bus, interval, warn, critical, core.SourceDisk),
		mount:      mount,
	}
}

func (p *DiskPoller) Run(ctx context.Context) {
	for {
		event := p.checkDisk()
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

func (p *DiskPoller) checkDisk() *core.Event {
	value, err := p.readValue()
	if err != nil {
		return nil
	}

	switch p.state {
	case stateOK:
		if value >= p.Warn {
			p.state = stateWarn
			return p.makeDiskEvent(core.SeverityWarn, value)
		}
	case stateWarn:
		if value >= p.Critical {
			p.state = stateCritical
			return p.makeDiskEvent(core.SeverityCritical, value)
		}
		if value < p.Warn {
			p.state = stateOK
			return p.makeDiskEvent(core.SeverityRecovered, value)
		}
	case stateCritical:
		if value < p.Warn {
			p.state = stateOK
			return p.makeDiskEvent(core.SeverityRecovered, value)
		}
	}

	return nil
}

func (p *DiskPoller) readValue() (float64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(p.mount, &st); err != nil {
		return 0, err
	}
	total := uint64(st.Frsize) * st.Blocks
	free := uint64(st.Frsize) * st.Bavail
	if total == 0 {
		return 0, nil
	}
	return float64(total-free) / float64(total) * 100.0, nil
}

func (p *DiskPoller) makeDiskEvent(severity string, value float64) *core.Event {
	return &core.Event{
		Source:    p.source,
		EventType: fmt.Sprintf("%s_%s", p.source, severity),
		Severity:  severity,
		Title:     fmt.Sprintf("Disk %s", severity),
		Message:   fmt.Sprintf("Mount '%s' at %.1f%% (warn=%.0f%%, crit=%.0f%%)", p.mount, value, p.Warn, p.Critical),
		Timestamp: time.Now(),
	}
}
