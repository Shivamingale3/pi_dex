package health

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Shivamingale3/pi_dex/internal/config"
	"github.com/Shivamingale3/pi_dex/internal/core"
)

// State is the health of one check.
type State int

const (
	StateUnknown State = iota
	StateHealthy
	StateFailing // failed, but fewer than the configured consecutive failures
	StateDown
)

func (s State) String() string {
	switch s {
	case StateHealthy:
		return "healthy"
	case StateFailing:
		return "failing"
	case StateDown:
		return "down"
	}
	return "unknown"
}

// Transition is what an observation changed, if anything worth alerting.
type Transition int

const (
	NoAlert Transition = iota
	WentDown
	Recovered
)

// tracker is the per-check state machine. It alerts once when a check reaches
// the failure threshold and once when it recovers; the first result after
// startup sets the state silently.
type tracker struct {
	threshold    int
	state        State
	consecutive  int
	failingSince time.Time
}

func newTracker(threshold int) *tracker {
	if threshold < 1 {
		threshold = 1
	}
	return &tracker{threshold: threshold}
}

// observe records one result. On Recovered, downtime is measured from the
// first failure of the streak.
func (t *tracker) observe(healthy bool, now time.Time) (tr Transition, downtime time.Duration) {
	if healthy {
		wasDown := t.state == StateDown
		t.state = StateHealthy
		t.consecutive = 0
		if wasDown {
			return Recovered, now.Sub(t.failingSince)
		}
		return NoAlert, 0
	}

	if t.consecutive == 0 {
		t.failingSince = now
	}
	t.consecutive++
	if t.state == StateDown {
		return NoAlert, 0
	}
	if t.consecutive >= t.threshold {
		t.state = StateDown
		return WentDown, 0
	}
	t.state = StateFailing
	return NoAlert, 0
}

// Monitor runs every configured check in its own goroutine and publishes
// HEALTH_DOWN / HEALTH_RECOVERED events.
type Monitor struct {
	bus    *core.EventBus
	checks []config.HealthCheckConfig
}

func NewMonitor(bus *core.EventBus, checks []config.HealthCheckConfig) *Monitor {
	return &Monitor{bus: bus, checks: checks}
}

// Run blocks until ctx is cancelled and all checks have stopped.
func (m *Monitor) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, hc := range m.checks {
		wg.Add(1)
		go func(hc config.HealthCheckConfig) {
			defer wg.Done()
			m.runCheck(ctx, hc.WithDefaults())
		}(hc)
	}
	wg.Wait()
}

func (m *Monitor) runCheck(ctx context.Context, hc config.HealthCheckConfig) {
	t := newTracker(hc.Failures)
	interval := time.Duration(hc.Interval) * time.Second
	log.Printf("health: %s (%s) every %s, alert after %d failures", hc.Name, hc.Type, interval, hc.Failures)

	for {
		err := Check(ctx, hc)
		if ctx.Err() != nil {
			return
		}
		prev := t.state
		tr, downtime := t.observe(err == nil, time.Now())
		if t.state != prev {
			if err != nil {
				log.Printf("health: %s %s -> %s: %v", hc.Name, prev, t.state, err)
			} else {
				log.Printf("health: %s %s -> %s", hc.Name, prev, t.state)
			}
		}

		switch tr {
		case WentDown:
			diag := Diagnose(ctx, hc, err)
			if ctx.Err() != nil {
				return
			}
			m.bus.Publish(DownEvent(hc, t.consecutive, diag, time.Now()))
		case Recovered:
			m.bus.Publish(RecoveredEvent(hc, downtime, time.Now()))
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

// Target describes what a check probes, e.g. "redis 100.83.1.25:6379".
func Target(hc config.HealthCheckConfig) string {
	switch hc.Type {
	case config.HealthHTTP, config.HealthTCP, config.HealthRedis:
		return hc.Type + " " + hc.Address
	case config.HealthSystemd:
		return "systemd " + config.UnitName(hc.Unit)
	case config.HealthDocker:
		return "docker " + hc.Container
	case config.HealthCommand:
		return "command"
	case config.HealthInterface:
		return "interface " + hc.Interface
	}
	return hc.Type
}

func DownEvent(hc config.HealthCheckConfig, failures int, d Diagnosis, now time.Time) core.Event {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s) is DOWN after %d consecutive failures", hc.Name, Target(hc), failures)
	if len(d.Findings) > 0 {
		b.WriteString("\n\nCause:")
		for _, f := range d.Findings {
			b.WriteString("\n• " + f)
		}
	}
	if d.LogSource != "" && d.Logs != "" {
		fmt.Fprintf(&b, "\n\nLast log lines (%s):", d.LogSource)
	}
	return core.Event{
		Source:    core.SourceHealthPrefix + hc.Name,
		EventType: core.EventHealthDown,
		Severity:  core.SeverityCritical,
		Title:     "Health Check Down: " + hc.Name,
		Message:   b.String(),
		Details:   d.Logs,
		Timestamp: now,
	}
}

func RecoveredEvent(hc config.HealthCheckConfig, downtime time.Duration, now time.Time) core.Event {
	return core.Event{
		Source:    core.SourceHealthPrefix + hc.Name,
		EventType: core.EventHealthRecovered,
		Severity:  core.SeverityRecovered,
		Title:     "Health Check Recovered: " + hc.Name,
		Message:   fmt.Sprintf("%s (%s) is healthy again after %s down", hc.Name, Target(hc), formatDuration(downtime)),
		Timestamp: now,
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "less than a second"
	}
	return d.Round(time.Second).String()
}

// Result is the outcome of RunOnce.
type Result struct {
	Err       error
	Elapsed   time.Duration
	Diagnosis Diagnosis
}

// RunOnce runs a check and, if it fails, its diagnosis. Used by
// `pidex test --health` and the setup wizard; it never sends notifications.
func RunOnce(ctx context.Context, hc config.HealthCheckConfig) Result {
	hc = hc.WithDefaults()
	start := time.Now()
	err := Check(ctx, hc)
	r := Result{Err: err, Elapsed: time.Since(start)}
	if err != nil {
		r.Diagnosis = Diagnose(ctx, hc, err)
	}
	return r
}

// String renders the result for terminals.
func (r Result) String() string {
	if r.Err == nil {
		return fmt.Sprintf("HEALTHY (%s)\n", r.Elapsed.Round(time.Millisecond))
	}
	return fmt.Sprintf("FAILED (%s)\n%s", r.Elapsed.Round(time.Millisecond), r.Diagnosis.String())
}
