package pollers

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v4/cpu"

	"PI_DEX/internal/core"
)

type CPUPoller struct {
	WarnThreshold float64
	Interval      time.Duration
}

func NewCPUPoller() *CPUPoller {
	return &CPUPoller{
		WarnThreshold: 80,
		Interval:      15 * time.Second,
	}
}

func (p *CPUPoller) Start(bus *core.EventBus) {

	hostname, _ := os.Hostname()

	alertSent := false

	for {

		usage, err := cpu.Percent(
			time.Second,
			false,
		)

		if err == nil && len(usage) > 0 {

			value := usage[0]

			if value >= p.WarnThreshold {

				if !alertSent {

					bus.Events <- core.Event{
						ID:        uuid.NewString(),
						Hostname:  hostname,
						Source:    "cpu",
						EventType: "CPU_HIGH",
						Severity:  core.WARN,
						Title:     "CPU Usage High",
						Message: fmt.Sprintf(
							"CPU usage %.2f%% (threshold %.0f%%)",
							value,
							p.WarnThreshold,
						),
						Timestamp: time.Now(),
					}

					alertSent = true
				}

			} else {
				if alertSent {
					bus.Events <- core.Event{
						ID:        uuid.NewString(),
						Hostname:  hostname,
						Source:    "cpu",
						EventType: "CPU_RECOVERED",
						Severity:  core.INFO,
						Title:     "CPU Recovered",
						Message: fmt.Sprintf(
							"CPU recovered to %.2f%% (threshold %.0f%%)",
							value,
							p.WarnThreshold,
						),
						Timestamp: time.Now(),
					}

					alertSent = false
				}
			}
		}

		time.Sleep(p.Interval)
	}
}
