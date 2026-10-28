package health

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Shivamingale3/pi_dex/internal/config"
)

const procTCP = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:18EB 00000000:0000 0A 00000000:00000000 00:00000000 00000000   999        0 1234 1 0000000000000000 100 0 0 10 0
   1: 00000000:0016 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 2345 1 0000000000000000 100 0 0 10 0
   2: 0100007F:18EB 0100007F:D431 01 00000000:00000000 00:00000000 00000000   999        0 3456 1 0000000000000000 20 4 30 10 -1
`

const procTCP6 = `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000001000000:18EB 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000   999        0 4567 1 0000000000000000 100 0 0 10 0
   1: 0000000000000000FFFF00001953640A:1F90 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 5678 1 0000000000000000 100 0 0 10 0
`

func TestParseProcNetTCP(t *testing.T) {
	got := parseProcNetTCP(strings.NewReader(procTCP))
	want := []netip.AddrPort{
		netip.MustParseAddrPort("127.0.0.1:6379"),
		netip.MustParseAddrPort("0.0.0.0:22"),
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v (established sockets must be skipped)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	got6 := parseProcNetTCP(strings.NewReader(procTCP6))
	want6 := []netip.AddrPort{
		netip.MustParseAddrPort("[::1]:6379"),
		netip.MustParseAddrPort("10.100.83.25:8080"), // IPv4-mapped, unmapped
	}
	if len(got6) != 2 || got6[0] != want6[0] || got6[1] != want6[1] {
		t.Errorf("tcp6 = %v, want %v", got6, want6)
	}
}

func TestListenerFinding(t *testing.T) {
	sockets := []netip.AddrPort{
		netip.MustParseAddrPort("127.0.0.1:6379"),
		netip.MustParseAddrPort("[::1]:6379"),
		netip.MustParseAddrPort("0.0.0.0:22"),
	}
	ts := netip.MustParseAddr("100.83.1.25")

	if got := listenerFinding(ts, 6379, sockets); got != "port 6379 is listening on 127.0.0.1:6379, [::1]:6379 only, not on 100.83.1.25" {
		t.Errorf("wrong bind: %q", got)
	}
	if got := listenerFinding(ts, 22, sockets); got != "port 22 is listening on 0.0.0.0:22" {
		t.Errorf("wildcard: %q", got)
	}
	if got := listenerFinding(ts, 8080, sockets); got != "nothing is listening on port 8080" {
		t.Errorf("none: %q", got)
	}
}

func TestTargetAddr(t *testing.T) {
	cases := []struct {
		hc   config.HealthCheckConfig
		ip   string
		port int
	}{
		{config.HealthCheckConfig{Type: config.HealthRedis, Address: "100.83.1.25:6379"}, "100.83.1.25", 6379},
		{config.HealthCheckConfig{Type: config.HealthHTTP, Address: "https://10.0.0.5/health"}, "10.0.0.5", 443},
		{config.HealthCheckConfig{Type: config.HealthHTTP, Address: "http://localhost:8080/"}, "127.0.0.1", 8080},
		{config.HealthCheckConfig{Type: config.HealthTCP, Address: "[::1]:22"}, "::1", 22},
		{config.HealthCheckConfig{Type: config.HealthTCP, Address: "example.com:22"}, "", 22},
	}
	for _, c := range cases {
		ip, port := targetAddr(c.hc)
		gotIP := ""
		if ip.IsValid() {
			gotIP = ip.String()
		}
		if gotIP != c.ip || port != c.port {
			t.Errorf("%s: got %q %d, want %q %d", c.hc.Address, gotIP, port, c.ip, c.port)
		}
	}
}

func TestDiagnoseReportsMissingIPAndWrongBind(t *testing.T) {
	dir := t.TempDir()
	tcp := filepath.Join(dir, "tcp")
	os.WriteFile(tcp, []byte(procTCP), 0644)
	old := procNetFiles
	procNetFiles = []string{tcp}
	defer func() { procNetFiles = old }()

	// 192.0.2.0/24 is TEST-NET-1 and never assigned to a real interface.
	hc := config.HealthCheckConfig{Name: "redis", Type: config.HealthRedis, Address: "192.0.2.10:6379", Interface: "lo"}
	d := Diagnose(context.Background(), hc, errors.New("timeout after 5s"))
	joined := strings.Join(d.Findings, "\n")
	for _, want := range []string{
		"timeout after 5s",
		"interface lo: UP, IPv4 127.0.0.1",
		"IP 192.0.2.10 is not assigned to any interface",
		"port 6379 is listening on 127.0.0.1:6379 only, not on 192.0.2.10",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("findings missing %q:\n%s", want, joined)
		}
	}
	if d.Findings[0] != "timeout after 5s" {
		t.Errorf("raw error must come first, got %q", d.Findings[0])
	}
}

func TestDiagnoseRemoteHostSkipsLocalSteps(t *testing.T) {
	hc := config.HealthCheckConfig{Name: "remote", Type: config.HealthTCP, Address: "192.0.2.10:22"}
	d := Diagnose(context.Background(), hc, errors.New("timeout after 5s"))
	if len(d.Findings) != 1 {
		t.Errorf("remote target without interface should only report the raw error, got %q", d.Findings)
	}
}

func TestFormatUnitShowAndTail(t *testing.T) {
	out := "ActiveState=failed\nSubState=failed\nResult=exit-code\nExecMainStatus=1\n"
	if got := formatUnitShow(out); got != "ActiveState=failed SubState=failed Result=exit-code ExecMainStatus=1" {
		t.Errorf("formatUnitShow = %q", got)
	}
	if got := tailBytes("aaaa\nbbbb\ncccc", 7); got != "cccc" {
		t.Errorf("tailBytes = %q", got)
	}
}
