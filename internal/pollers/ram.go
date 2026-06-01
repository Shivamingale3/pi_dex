package pollers

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v4/mem"

	"PI_DEX/internal/core"
)

type RAMPoller struct {
	WarnThreshold float64
	Interval      time.Duration
}

func NewRAMPoller() *RAMPoller {

	return &RAMPoller{
		WarnThreshold: 80,
		Interval:      30 * time.Second,
	}
}

func (p *RAMPoller) Start(bus *core.EventBus) {

	hostname, _ := os.Hostname()

	alertSent := false

	for {

		vm, err := mem.VirtualMemory()

		if err == nil {

			if vm.UsedPercent >= p.WarnThreshold {

				if !alertSent {

					bus.Events <- core.Event{
						ID:        uuid.NewString(),
						Hostname:  hostname,
						Source:    "ram",
						EventType: "RAM_HIGH",
						Severity:  core.WARN,
						Title:     "RAM Usage High",
						Message: fmt.Sprintf(
							"RAM usage %.2f%% (threshold %.0f%%)",
							vm.UsedPercent,
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
						Source:    "ram",
						EventType: "RAM_RECOVERED",
						Severity:  core.INFO,
						Title:     "RAM Recovered",
						Message: fmt.Sprintf(
							"RAM recovered to %.2f%% (threshold %.0f%%)",
							vm.UsedPercent,
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
