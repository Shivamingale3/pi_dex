package poller

import (
	"bufio"
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/Shivamingale3/pi_dex/internal/core"
)

type RamPoller struct {
	BasePoller
}

func NewRamPoller(bus *core.EventBus, interval int, warn, critical float64) *RamPoller {
	return &RamPoller{
		BasePoller: New(bus, interval, warn, critical, core.SourceRAM),
	}
}

func (p *RamPoller) Run(ctx context.Context) {
	p.BasePoller.Run(ctx, p.readValue)
}

func (p *RamPoller) readValue() (float64, error) {
	total, available, err := readMeminfo()
	if err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, nil
	}
	return float64(total-available) / float64(total) * 100.0, nil
}

func readMeminfo() (total, available uint64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			total = parseMemValue(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			available = parseMemValue(line)
		}
	}
	return total, available, nil
}

func parseMemValue(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	v, _ := strconv.ParseUint(fields[1], 10, 64)
	return v
}
