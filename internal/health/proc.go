package health

import (
	"bufio"
	"encoding/hex"
	"io"
	"net/netip"
	"os"
	"strconv"
	"strings"
)

// procNetFiles are variables so tests can substitute fixtures.
var procNetFiles = []string{"/proc/net/tcp", "/proc/net/tcp6"}

// tcpStateListen is the LISTEN state in /proc/net/tcp.
const tcpStateListen = "0A"

// listeningSockets returns the local addresses of all listening TCP sockets.
func listeningSockets() []netip.AddrPort {
	var out []netip.AddrPort
	for _, p := range procNetFiles {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		out = append(out, parseProcNetTCP(f)...)
		f.Close()
	}
	return out
}

// parseProcNetTCP parses /proc/net/tcp or /proc/net/tcp6 and returns the
// local address of every socket in LISTEN state.
func parseProcNetTCP(r io.Reader) []netip.AddrPort {
	var out []netip.AddrPort
	scanner := bufio.NewScanner(r)
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 || fields[3] != tcpStateListen {
			continue
		}
		if ap, ok := parseHexAddrPort(fields[1]); ok {
			out = append(out, ap)
		}
	}
	return out
}

// parseHexAddrPort decodes "0100007F:18EB" (IPv4) or a 32-hex-digit IPv6
// address. The kernel prints each 32-bit word in host byte order, which is
// little-endian on every platform PiDex supports (amd64, arm64).
func parseHexAddrPort(s string) (netip.AddrPort, bool) {
	host, portHex, ok := strings.Cut(s, ":")
	if !ok {
		return netip.AddrPort{}, false
	}
	port, err := strconv.ParseUint(portHex, 16, 16)
	if err != nil {
		return netip.AddrPort{}, false
	}
	raw, err := hex.DecodeString(host)
	if err != nil || (len(raw) != 4 && len(raw) != 16) {
		return netip.AddrPort{}, false
	}
	for i := 0; i < len(raw); i += 4 {
		raw[i], raw[i+1], raw[i+2], raw[i+3] = raw[i+3], raw[i+2], raw[i+1], raw[i]
	}
	addr, ok := netip.AddrFromSlice(raw)
	if !ok {
		return netip.AddrPort{}, false
	}
	return netip.AddrPortFrom(addr.Unmap(), uint16(port)), true
}
