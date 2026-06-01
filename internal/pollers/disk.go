package pollers

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v4/disk"

	"PI_DEX/internal/core"
)

type DiskPoller struct {
	WarnThreshold float64
	Interval      time.Duration
}

func NewDiskPoller() *DiskPoller {

	return &DiskPoller{
		WarnThreshold: 80,
		Interval:      5 * time.Minute,
	}
}

func (p *DiskPoller) Start(bus *core.EventBus) {

	hostname, _ := os.Hostname()

	alertSent := false

	for {

		usage, err := disk.Usage("/")

		if err == nil {

			if usage.UsedPercent >= p.WarnThreshold {

				if !alertSent {

					bus.Events <- core.Event{
						ID:        uuid.NewString(),
						Hostname:  hostname,
						Source:    "disk",
						EventType: "DISK_WARN",
						Severity:  core.WARN,
						Title:     "Disk Usage High",
						Message: fmt.Sprintf(
							"Disk usage %.2f%% (threshold %.0f%%)",
							usage.UsedPercent,
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
						Source:    "disk",
						EventType: "DISK_RECOVERED",
						Severity:  core.INFO,
						Title:     "Disk Recovered",
						Message: fmt.Sprintf(
							"Disk recovered to %.2f%% (threshold %.0f%%)",
							usage.UsedPercent,
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
