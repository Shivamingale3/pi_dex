package health

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Shivamingale3/pi_dex/internal/config"
)

const (
	logLines        = 10
	diagStepTimeout = 5 * time.Second
	// Keeps the whole alert well under Telegram's 4096 character limit.
	maxLogBytes = 2500
)

// Diagnosis explains why a check failed.
type Diagnosis struct {
	Findings  []string // one line per finding, raw error first
	LogSource string   // e.g. "journalctl -u redis.service"
	Logs      string   // last log lines, may be empty
}

// Diagnose collects everything that may explain checkErr. Each step that has
// nothing to report is skipped; steps that fail report their own error.
func Diagnose(ctx context.Context, hc config.HealthCheckConfig, checkErr error) Diagnosis {
	var d Diagnosis
	if checkErr != nil {
		d.Findings = append(d.Findings, checkErr.Error())
	}

	unit := config.UnitName(hc.Unit)
	if unit != "" {
		d.Findings = append(d.Findings, unitFinding(ctx, unit))
	}
	if hc.Container != "" {
		d.Findings = append(d.Findings, containerFinding(ctx, hc.Container))
	}

	ip, port := targetAddr(hc)
	local := false
	d.Findings, local = appendAddressFindings(d.Findings, hc.Interface, ip)
	if local && port != 0 && (hc.Type == config.HealthTCP || hc.Type == config.HealthRedis || hc.Type == config.HealthHTTP) {
		d.Findings = append(d.Findings, listenerFinding(ip, port, listeningSockets()))
	}

	switch {
	case unit != "":
		d.LogSource = "journalctl -u " + unit
		d.Logs = unitLogs(ctx, unit)
	case hc.Container != "":
		d.LogSource = "docker logs " + hc.Container
		d.Logs = dockerLogs(ctx, hc.Container)
	}
	d.Logs = tailBytes(strings.TrimRight(d.Logs, "\n"), maxLogBytes)
	return d
}

// String renders the diagnosis for terminals.
func (d Diagnosis) String() string {
	var b strings.Builder
	for _, f := range d.Findings {
		b.WriteString("  - " + f + "\n")
	}
	if d.LogSource != "" {
		fmt.Fprintf(&b, "\n  Last %d log lines (%s):\n", logLines, d.LogSource)
		if d.Logs == "" {
			b.WriteString("    (none)\n")
		}
		for _, l := range strings.Split(d.Logs, "\n") {
			if d.Logs != "" {
				b.WriteString("    " + l + "\n")
			}
		}
	}
	return b.String()
}

func unitFinding(ctx context.Context, unit string) string {
	ctx, cancel := context.WithTimeout(ctx, diagStepTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "systemctl", "show", "-p", "ActiveState,SubState,Result,ExecMainStatus", unit).Output()
	if err != nil {
		return fmt.Sprintf("unit %s: systemctl show failed: %v", unit, err)
	}
	return fmt.Sprintf("unit %s: %s", unit, formatUnitShow(string(out)))
}

// formatUnitShow turns "Key=Value" lines from systemctl show into one line.
func formatUnitShow(out string) string {
	var parts []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			parts = append(parts, line)
		}
	}
	if len(parts) == 0 {
		return "no state reported"
	}
	return strings.Join(parts, " ")
}

func containerFinding(ctx context.Context, name string) string {
	ctx, cancel := context.WithTimeout(ctx, diagStepTimeout)
	defer cancel()
	st, err := inspectContainer(ctx, name)
	if err != nil {
		return fmt.Sprintf("container %s: %v", name, err)
	}
	health := st.Health
	if health == "" {
		health = "none"
	}
	s := fmt.Sprintf("container %s: state=%s exit_code=%d health=%s", name, st.Status, st.ExitCode, health)
	if st.Error != "" {
		s += " error=" + st.Error
	}
	return s
}

// targetAddr extracts the IP literal and port a check connects to. The IP is
// invalid when the address uses a hostname (other than localhost).
func targetAddr(hc config.HealthCheckConfig) (netip.Addr, int) {
	var host, portStr string
	switch hc.Type {
	case config.HealthHTTP:
		u, err := url.Parse(hc.Address)
		if err != nil {
			return netip.Addr{}, 0
		}
		host, portStr = u.Hostname(), u.Port()
		if portStr == "" {
			portStr = "80"
			if u.Scheme == "https" {
				portStr = "443"
			}
		}
	case config.HealthTCP, config.HealthRedis:
		var err error
		host, portStr, err = net.SplitHostPort(hc.Address)
		if err != nil {
			return netip.Addr{}, 0
		}
	default:
		return netip.Addr{}, 0
	}
	port, _ := strconv.Atoi(portStr)
	if host == "localhost" {
		host = "127.0.0.1"
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, port
	}
	return ip.Unmap(), port
}

// appendAddressFindings reports the state of the configured interface and
// whether the target IP is currently assigned to any interface. It returns
// whether the target is (or is expected to be) on this host.
func appendAddressFindings(findings []string, ifaceName string, ip netip.Addr) ([]string, bool) {
	owners := localIPOwners()

	if ifaceName != "" {
		findings = append(findings, interfaceFinding(ifaceName))
	}
	if !ip.IsValid() {
		return findings, false
	}
	if ip.IsLoopback() || ip.IsUnspecified() {
		return findings, true
	}
	if owner, ok := owners[ip]; ok {
		if ifaceName != "" && owner != ifaceName {
			findings = append(findings, fmt.Sprintf("IP %s is on %s, not %s", ip, owner, ifaceName))
		}
		return findings, true
	}
	if ifaceName != "" {
		findings = append(findings, fmt.Sprintf("IP %s is not assigned to any interface", ip))
		return findings, true
	}
	return findings, false
}

func interfaceFinding(name string) string {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return fmt.Sprintf("interface %s: not found", name)
	}
	state := "UP"
	if iface.Flags&net.FlagUp == 0 {
		state = "DOWN"
	}
	addrs := ipv4Addrs(iface)
	if len(addrs) == 0 {
		return fmt.Sprintf("interface %s: %s, no IPv4 address", name, state)
	}
	return fmt.Sprintf("interface %s: %s, IPv4 %s", name, state, strings.Join(addrs, ", "))
}

func localIPOwners() map[netip.Addr]string {
	owners := map[netip.Addr]string{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return owners
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok {
				if ip, ok := netip.AddrFromSlice(ipn.IP); ok {
					owners[ip.Unmap()] = iface.Name
				}
			}
		}
	}
	return owners
}

// listenerFinding reports which addresses listen on port, e.g.
// "port 6379 is listening on 127.0.0.1:6379 only".
func listenerFinding(target netip.Addr, port int, sockets []netip.AddrPort) string {
	seen := map[string]bool{}
	var on []string
	matches := false
	for _, s := range sockets {
		if int(s.Port()) != port {
			continue
		}
		if s.Addr().IsUnspecified() || !target.IsValid() || s.Addr() == target {
			matches = true
		}
		if str := s.String(); !seen[str] {
			seen[str] = true
			on = append(on, str)
		}
	}
	sort.Strings(on)
	switch {
	case len(on) == 0:
		return fmt.Sprintf("nothing is listening on port %d", port)
	case matches:
		return fmt.Sprintf("port %d is listening on %s", port, strings.Join(on, ", "))
	default:
		return fmt.Sprintf("port %d is listening on %s only, not on %s", port, strings.Join(on, ", "), target)
	}
}

func unitLogs(ctx context.Context, unit string) string {
	ctx, cancel := context.WithTimeout(ctx, diagStepTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "journalctl", "-u", unit, "-n", strconv.Itoa(logLines), "--no-pager", "-q").Output()
	if err != nil && len(out) == 0 {
		return fmt.Sprintf("(journalctl failed: %v; is pidex in the systemd-journal group?)", err)
	}
	return string(out)
}

func dockerLogs(ctx context.Context, name string) string {
	ctx, cancel := context.WithTimeout(ctx, diagStepTimeout)
	defer cancel()
	logs, err := containerLogs(ctx, name, logLines)
	if err != nil {
		return fmt.Sprintf("(docker logs failed: %v)", err)
	}
	return logs
}

// tailBytes keeps the last n bytes of s, starting at a line boundary.
func tailBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[len(s)-n:]
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	return s
}
