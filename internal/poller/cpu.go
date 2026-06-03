package poller

import (
	"bufio"
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/leadows/pi_dex/internal/core"
)

type CpuPoller struct {
	BasePoller
	prevIdle  uint64
	prevTotal uint64
}

func NewCpuPoller(bus *core.EventBus, interval int, warn, critical float64) *CpuPoller {
	idle, total := readCPU()
	return &CpuPoller{
		BasePoller: New(bus, interval, warn, critical, core.SourceCPU),
		prevIdle:   idle,
		prevTotal:  total,
	}
}

func (p *CpuPoller) Run(ctx context.Context) {
	p.BasePoller.Run(ctx, p.readValue)
}

func (p *CpuPoller) readValue() (float64, error) {
	idle, total := readCPU()
	deltaTotal := total - p.prevTotal
	deltaIdle := idle - p.prevIdle
	p.prevIdle = idle
	p.prevTotal = total

	if deltaTotal == 0 {
		return 0, nil
	}

	return (1.0 - float64(deltaIdle)/float64(deltaTotal)) * 100.0, nil
}

func readCPU() (idle, total uint64) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, 0
	}

	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 {
		return 0, 0
	}

	for i := 1; i < len(fields); i++ {
		v, _ := strconv.ParseUint(fields[i], 10, 64)
		total += v
	}
	idle, _ = strconv.ParseUint(fields[4], 10, 64)
	return idle, total
}
