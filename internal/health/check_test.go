package health

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Shivamingale3/pi_dex/internal/config"
)

func check(t *testing.T, hc config.HealthCheckConfig) error {
	t.Helper()
	if hc.Name == "" {
		hc.Name = "t"
	}
	return Check(context.Background(), hc.WithDefaults())
}

func TestTCPCheck(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()

	if err := check(t, config.HealthCheckConfig{Type: config.HealthTCP, Address: addr}); err != nil {
		t.Errorf("listening port: %v", err)
	}
	ln.Close()
	err = check(t, config.HealthCheckConfig{Type: config.HealthTCP, Address: addr})
	if err == nil || err.Error() != "connection refused" {
		t.Errorf("closed port: err = %v, want connection refused", err)
	}
}

func TestHTTPCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer s3cret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/broken" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/slow" {
			time.Sleep(1500 * time.Millisecond)
		}
	}))
	defer srv.Close()
	headers := map[string]string{"Authorization": "Bearer s3cret"}

	if err := check(t, config.HealthCheckConfig{Type: config.HealthHTTP, Address: srv.URL + "/", Headers: headers}); err != nil {
		t.Errorf("healthy: %v", err)
	}
	err := check(t, config.HealthCheckConfig{Type: config.HealthHTTP, Address: srv.URL + "/broken", Headers: headers})
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("500: err = %v", err)
	}
	err = check(t, config.HealthCheckConfig{Type: config.HealthHTTP, Address: srv.URL + "/"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Errorf("missing header: err = %v", err)
	}
	if err := check(t, config.HealthCheckConfig{Type: config.HealthHTTP, Address: srv.URL + "/broken", Headers: headers, ExpectStatus: 500}); err != nil {
		t.Errorf("expect_status 500: %v", err)
	}
	err = check(t, config.HealthCheckConfig{Type: config.HealthHTTP, Address: srv.URL + "/slow", Headers: headers, Timeout: 1})
	if err == nil || err.Error() != "timeout after 1s" {
		t.Errorf("slow: err = %v, want timeout after 1s", err)
	}
}

// fakeRedis speaks just enough RESP for AUTH and PING.
func fakeRedis(t *testing.T, password string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				authed := password == ""
				for {
					args, err := readCommand(r)
					if err != nil {
						return
					}
					switch strings.ToUpper(args[0]) {
					case "AUTH":
						if args[len(args)-1] == password {
							authed = true
							c.Write([]byte("+OK\r\n"))
						} else {
							c.Write([]byte("-WRONGPASS invalid username-password pair or user is disabled.\r\n"))
						}
					case "PING":
						if !authed {
							c.Write([]byte("-NOAUTH Authentication required.\r\n"))
						} else {
							c.Write([]byte("+PONG\r\n"))
						}
					}
				}
			}(c)
		}
	}()
	return ln.Addr().String()
}

func readCommand(r *bufio.Reader) ([]string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	var n int
	if _, err := fmt.Sscanf(line, "*%d", &n); err != nil {
		return nil, err
	}
	args := make([]string, n)
	for i := range args {
		if _, err := r.ReadString('\n'); err != nil { // $len
			return nil, err
		}
		v, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		args[i] = strings.TrimRight(v, "\r\n")
	}
	return args, nil
}

func TestRedisCheck(t *testing.T) {
	open := fakeRedis(t, "")
	if err := check(t, config.HealthCheckConfig{Type: config.HealthRedis, Address: open}); err != nil {
		t.Errorf("no auth: %v", err)
	}

	secured := fakeRedis(t, "hunter2")
	if err := check(t, config.HealthCheckConfig{Type: config.HealthRedis, Address: secured, Password: "hunter2"}); err != nil {
		t.Errorf("auth ok: %v", err)
	}
	if err := check(t, config.HealthCheckConfig{Type: config.HealthRedis, Address: secured, Username: "mon", Password: "hunter2"}); err != nil {
		t.Errorf("acl auth ok: %v", err)
	}
	err := check(t, config.HealthCheckConfig{Type: config.HealthRedis, Address: secured, Password: "wrong"})
	if err == nil || !strings.HasPrefix(err.Error(), "AUTH failed: WRONGPASS") {
		t.Errorf("wrong password: err = %v", err)
	}
	err = check(t, config.HealthCheckConfig{Type: config.HealthRedis, Address: secured})
	if err == nil || !strings.Contains(err.Error(), "NOAUTH") {
		t.Errorf("missing password: err = %v", err)
	}
}

func TestCommandCheck(t *testing.T) {
	if err := check(t, config.HealthCheckConfig{Type: config.HealthCommand, Command: "true"}); err != nil {
		t.Errorf("true: %v", err)
	}
	err := check(t, config.HealthCheckConfig{Type: config.HealthCommand, Command: "echo disk full; exit 3"})
	if err == nil || err.Error() != "exit code 3: disk full" {
		t.Errorf("exit 3: err = %v", err)
	}
	start := time.Now()
	err = check(t, config.HealthCheckConfig{Type: config.HealthCommand, Command: "sleep 30 & sleep 30", Timeout: 1})
	if err == nil || err.Error() != "timeout after 1s" {
		t.Errorf("timeout: err = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("timeout took %v; child processes were not killed", elapsed)
	}
}

func TestInterfaceCheck(t *testing.T) {
	if err := check(t, config.HealthCheckConfig{Type: config.HealthInterface, Interface: "lo"}); err != nil {
		t.Errorf("lo: %v", err)
	}
	err := check(t, config.HealthCheckConfig{Type: config.HealthInterface, Interface: "pidex-missing0"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("missing: err = %v", err)
	}
}

func TestDockerCheckWithoutSocket(t *testing.T) {
	old := dockerSocket
	dockerSocket = t.TempDir() + "/missing.sock"
	defer func() { dockerSocket = old }()
	err := check(t, config.HealthCheckConfig{Type: config.HealthDocker, Container: "web"})
	if err == nil || !strings.Contains(err.Error(), "docker API") {
		t.Errorf("err = %v", err)
	}
}

func TestDockerCheckAgainstFakeAPI(t *testing.T) {
	sock := t.TempDir() + "/docker.sock"
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/containers/web/json":
			w.Write([]byte(`{"State":{"Status":"running","Running":true,"ExitCode":0,"Health":{"Status":"unhealthy"}}}`))
		case "/containers/db/json":
			w.Write([]byte(`{"State":{"Status":"exited","Running":false,"ExitCode":137}}`))
		case "/containers/ok/json":
			w.Write([]byte(`{"State":{"Status":"running","Running":true}}`))
		default:
			http.NotFound(w, r)
		}
	})}
	go srv.Serve(ln)
	defer srv.Close()

	old := dockerSocket
	dockerSocket = sock
	defer func() { dockerSocket = old }()

	cases := map[string]string{
		"ok":      "",
		"web":     "container web health is unhealthy",
		"db":      "container db is exited (exit code 137)",
		"missing": "container missing not found",
	}
	for name, want := range cases {
		err := check(t, config.HealthCheckConfig{Type: config.HealthDocker, Container: name})
		if want == "" {
			if err != nil {
				t.Errorf("%s: %v", name, err)
			}
			continue
		}
		if err == nil || err.Error() != want {
			t.Errorf("%s: err = %v, want %q", name, err, want)
		}
	}
}

func TestDemuxDockerLogs(t *testing.T) {
	frame := func(stream byte, s string) []byte {
		n := len(s)
		return append([]byte{stream, 0, 0, 0, byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}, s...)
	}
	b := append(frame(1, "out line\n"), frame(2, "err line\n")...)
	if got := demuxDockerLogs(b); got != "out line\nerr line\n" {
		t.Errorf("demux = %q", got)
	}
	if got := demuxDockerLogs([]byte("tty output\n")); got != "tty output\n" {
		t.Errorf("tty = %q", got)
	}
}
