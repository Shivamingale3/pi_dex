package health

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// dockerSocket is a variable so tests can point it elsewhere.
var dockerSocket = "/var/run/docker.sock"

type containerState struct {
	Status   string
	Running  bool
	ExitCode int
	Health   string // empty when the container has no healthcheck
	Error    string
}

func dockerClient() *http.Client {
	return &http.Client{Transport: &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", dockerSocket)
		},
		DisableKeepAlives: true,
	}}
}

func dockerGet(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker"+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := dockerClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker API: %w", unwrapURLError(err))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no such container")
	}
	if resp.StatusCode != http.StatusOK {
		var msg struct{ Message string }
		json.Unmarshal(body, &msg)
		return nil, fmt.Errorf("docker API: HTTP %d %s", resp.StatusCode, msg.Message)
	}
	return body, nil
}

func inspectContainer(ctx context.Context, name string) (containerState, error) {
	body, err := dockerGet(ctx, "/containers/"+url.PathEscape(name)+"/json")
	if err != nil {
		if err.Error() == "no such container" {
			return containerState{}, fmt.Errorf("container %s not found", name)
		}
		return containerState{}, err
	}
	var raw struct {
		State struct {
			Status   string
			Running  bool
			ExitCode int
			Error    string
			Health   *struct{ Status string }
		}
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return containerState{}, fmt.Errorf("docker API: %v", err)
	}
	st := containerState{
		Status:   raw.State.Status,
		Running:  raw.State.Running,
		ExitCode: raw.State.ExitCode,
		Error:    raw.State.Error,
	}
	if raw.State.Health != nil {
		st.Health = raw.State.Health.Status
	}
	return st, nil
}

func containerLogs(ctx context.Context, name string, tail int) (string, error) {
	path := fmt.Sprintf("/containers/%s/logs?stdout=1&stderr=1&tail=%d", url.PathEscape(name), tail)
	body, err := dockerGet(ctx, path)
	if err != nil {
		return "", err
	}
	return demuxDockerLogs(body), nil
}

// demuxDockerLogs strips the 8-byte stream headers Docker adds to logs of
// containers without a TTY. Output of TTY containers is returned unchanged.
func demuxDockerLogs(b []byte) string {
	var out strings.Builder
	for len(b) >= 8 && b[0] <= 2 && b[1] == 0 && b[2] == 0 && b[3] == 0 {
		n := int(binary.BigEndian.Uint32(b[4:8]))
		b = b[8:]
		if n > len(b) {
			n = len(b)
		}
		out.Write(b[:n])
		b = b[n:]
	}
	out.Write(b)
	return out.String()
}

func unwrapURLError(err error) error {
	if ue, ok := err.(*url.Error); ok {
		return ue.Err
	}
	return err
}
