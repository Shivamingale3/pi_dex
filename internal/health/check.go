// Package health runs active health checks (HTTP, TCP, Redis, systemd,
// Docker, command, interface) and alerts when a check goes down or recovers.
package health

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/Shivamingale3/pi_dex/internal/config"
)

// Check runs hc once. It returns nil when the target is healthy, otherwise an
// error whose text is the raw cause ("connection refused", "HTTP 500", ...).
func Check(ctx context.Context, hc config.HealthCheckConfig) error {
	hc = hc.WithDefaults()
	timeout := time.Duration(hc.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var err error
	switch hc.Type {
	case config.HealthHTTP:
		err = checkHTTP(ctx, hc)
	case config.HealthTCP:
		err = checkTCP(ctx, hc.Address)
	case config.HealthRedis:
		err = checkRedis(ctx, hc)
	case config.HealthSystemd:
		err = checkSystemd(ctx, hc.Unit)
	case config.HealthDocker:
		err = checkDocker(ctx, hc.Container)
	case config.HealthCommand:
		err = checkCommand(ctx, hc.Command)
	case config.HealthInterface:
		err = checkInterface(hc.Interface)
	default:
		return fmt.Errorf("unknown check type %q", hc.Type)
	}
	if err == nil {
		return nil
	}
	return errors.New(describeErr(err, ctx, timeout))
}

// describeErr turns low-level network errors into short readable causes.
func describeErr(err error, ctx context.Context, timeout time.Duration) string {
	var ne net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded), ctx.Err() == context.DeadlineExceeded,
		errors.As(err, &ne) && ne.Timeout():
		return fmt.Sprintf("timeout after %s", timeout)
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection refused"
	case errors.Is(err, syscall.ECONNRESET):
		return "connection reset by peer"
	case errors.Is(err, syscall.EHOSTUNREACH):
		return "host unreachable"
	case errors.Is(err, syscall.ENETUNREACH):
		return "network unreachable"
	case errors.Is(err, syscall.EADDRNOTAVAIL):
		return "address not available"
	}
	return err.Error()
}

func checkHTTP(ctx context.Context, hc config.HealthCheckConfig) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hc.Address, nil)
	if err != nil {
		return err
	}
	for k, v := range hc.Headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Transport: &http.Transport{
		Proxy:             nil,
		DisableKeepAlives: true,
	}}
	resp, err := client.Do(req)
	if err != nil {
		return unwrapURLError(err)
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
	resp.Body.Close()
	if resp.StatusCode != hc.ExpectStatus {
		return fmt.Errorf("HTTP %d (expected %d)", resp.StatusCode, hc.ExpectStatus)
	}
	return nil
}

func checkTCP(ctx context.Context, address string) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	return conn.Close()
}

func checkRedis(ctx context.Context, hc config.HealthCheckConfig) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", hc.Address)
	if err != nil {
		return err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}
	r := bufio.NewReader(conn)

	if hc.Password != "" {
		args := []string{"AUTH", hc.Password}
		if hc.Username != "" {
			args = []string{"AUTH", hc.Username, hc.Password}
		}
		reply, err := redisCommand(conn, r, args...)
		if err != nil {
			return err
		}
		if reply != "OK" {
			return fmt.Errorf("AUTH failed: %s", reply)
		}
	}

	reply, err := redisCommand(conn, r, "PING")
	if err != nil {
		return err
	}
	if reply != "PONG" {
		if strings.HasPrefix(reply, "NOAUTH") {
			return fmt.Errorf("AUTH failed: %s", reply)
		}
		return fmt.Errorf("PING returned %q", reply)
	}
	return nil
}

// redisCommand sends a RESP array and returns the reply as text. Error replies
// are returned as text (without the leading '-') rather than as an error, so
// the caller can tell protocol answers from connection failures.
func redisCommand(w io.Writer, r *bufio.Reader, args ...string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
		return "", err
	}

	line, err := readRESPLine(r)
	if err != nil {
		return "", err
	}
	if line == "" {
		return "", fmt.Errorf("empty reply from redis")
	}
	switch line[0] {
	case '+', '-':
		return line[1:], nil
	case '$':
		if line == "$-1" {
			return "", nil
		}
		return readRESPLine(r)
	}
	return "", fmt.Errorf("unexpected reply from redis: %q", line)
}

func readRESPLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "", fmt.Errorf("connection closed by redis")
		}
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func checkSystemd(ctx context.Context, unit string) error {
	unit = config.UnitName(unit)
	out, err := exec.CommandContext(ctx, "systemctl", "is-active", unit).Output()
	state := strings.TrimSpace(string(out))
	if state == "active" {
		return nil
	}
	if state == "" && err != nil {
		return fmt.Errorf("systemctl is-active %s: %v", unit, err)
	}
	return fmt.Errorf("%s is %s", unit, state)
}

func checkDocker(ctx context.Context, container string) error {
	st, err := inspectContainer(ctx, container)
	if err != nil {
		return err
	}
	if !st.Running {
		return fmt.Errorf("container %s is %s (exit code %d)", container, st.Status, st.ExitCode)
	}
	if st.Health != "" && st.Health != "healthy" {
		return fmt.Errorf("container %s health is %s", container, st.Health)
	}
	return nil
}

func checkCommand(ctx context.Context, command string) error {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	// Run in its own process group so a timeout kills children too, and do
	// not wait forever for pipes held open by them.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = time.Second

	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		msg := fmt.Sprintf("exit code %d", ee.ExitCode())
		if last := lastLine(string(out)); last != "" {
			msg += ": " + truncate(last, 200)
		}
		return errors.New(msg)
	}
	return err
}

func checkInterface(name string) error {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return fmt.Errorf("interface %s not found", name)
	}
	if iface.Flags&net.FlagUp == 0 {
		return fmt.Errorf("interface %s is down", name)
	}
	if len(ipv4Addrs(iface)) == 0 {
		return fmt.Errorf("interface %s has no IPv4 address", name)
	}
	return nil
}

func ipv4Addrs(iface *net.Interface) []string {
	addrs, err := iface.Addrs()
	if err != nil {
		return nil
	}
	var out []string
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil {
			out = append(out, ipn.IP.String())
		}
	}
	return out
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
