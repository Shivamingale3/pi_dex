package source

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/leadows/pi_dex/internal/core"
)

type DockerSource struct {
	bus   *core.EventBus
	watch []string
}

func NewDockerSource(bus *core.EventBus, watch []string) *DockerSource {
	return &DockerSource{bus: bus, watch: watch}
}

func (s *DockerSource) Run(ctx context.Context) error {
	client := &http.Client{
		Transport: &unixRoundTripper{},
	}

	for {
		req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost/v1.47/events", nil)
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("docker: connect: %v — retrying in 5s", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
			}
			continue
		}

		decoder := json.NewDecoder(resp.Body)
		for {
			var raw map[string]any
			if err := decoder.Decode(&raw); err != nil {
				resp.Body.Close()
				log.Printf("docker: decode error: %v — reconnecting in 5s", err)
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(5 * time.Second):
				}
				break
			}

			event := s.parseEvent(raw)
			if event != nil {
				s.bus.Publish(*event)
			}

			select {
			case <-ctx.Done():
				resp.Body.Close()
				return nil
			default:
			}
		}
	}
}

type unixRoundTripper struct{}

func (t *unixRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	conn, err := net.DialTimeout("unix", "/var/run/docker.sock", 5*time.Second)
	if err != nil {
		return nil, err
	}

	req.URL.Scheme = "http"
	req.URL.Host = "localhost"

	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		conn.Close()
		return nil, err
	}
	resp.Body = &closeOnce{ReadCloser: resp.Body, closer: conn}
	return resp, nil
}

type closeOnce struct {
	io.ReadCloser
	closer io.Closer
	once   sync.Once
}

func (c *closeOnce) Close() error {
	var err error
	c.once.Do(func() {
		err = c.closer.Close()
		if rcErr := c.ReadCloser.Close(); rcErr != nil && err == nil {
			err = rcErr
		}
	})
	return err
}

var dockerEventMap = map[string][3]string{
	"start":   {core.EventContainerStarted, core.SeverityInfo, "Container Started"},
	"stop":    {core.EventContainerStopped, core.SeverityWarn, "Container Stopped"},
	"die":     {core.EventContainerDied, core.SeverityCritical, "Container Died"},
	"restart": {core.EventContainerRestarted, core.SeverityInfo, "Container Restarted"},
}

func (s *DockerSource) parseEvent(raw map[string]any) *core.Event {
	etype, _ := raw["Type"].(string)
	if etype != "container" {
		return nil
	}

	action, _ := raw["Action"].(string)
	mapping, ok := dockerEventMap[action]
	if !ok {
		return nil
	}

	actor, _ := raw["Actor"].(map[string]any)
	attrs, _ := actor["Attributes"].(map[string]any)
	name, _ := attrs["name"].(string)
	if name == "" {
		name = "unknown"
	}
	id, _ := raw["id"].(string)
	if len(id) > 12 {
		id = id[:12]
	}

	if len(s.watch) > 0 && !matchContainer(name, s.watch) {
		return nil
	}

	eventType, severity, title := mapping[0], mapping[1], mapping[2]

	ts := time.Now()
	if t, ok := raw["time"].(float64); ok {
		ts = time.Unix(int64(t), 0)
	}

	return &core.Event{
		Source:    core.SourceDocker,
		EventType: eventType,
		Severity:  severity,
		Title:     title,
		Message:   fmt.Sprintf("Container '%s' (%s)", name, id),
		Timestamp: ts,
	}
}

func matchContainer(name string, patterns []string) bool {
	for _, p := range patterns {
		if matched, _ := path.Match(p, name); matched {
			return true
		}
	}
	return false
}
