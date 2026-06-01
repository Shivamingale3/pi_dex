package sources

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
	"github.com/google/uuid"

	"PI_DEX/internal/core"
)

type DockerSource struct {
}

func NewDockerSource() *DockerSource {
	return &DockerSource{}
}

func (d *DockerSource) Start(bus *core.EventBus) error {

	hostname, _ := os.Hostname()

	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)

	if err != nil {
		return err
	}

	msgs, errs := cli.Events(
		context.Background(),
		events.ListOptions{},
	)

	log.Println("Docker source started")

	for {

		select {

		case err := <-errs:

			if err != nil {
				log.Printf(
					"docker event error: %v",
					err,
				)
			}

		case msg := <-msgs:
			log.Printf(
				"Docker Event -> Type=%s Action=%s Name=%s",
				msg.Type,
				msg.Action,
				msg.Actor.Attributes["name"],
			)
			event := convertDockerEvent(
				hostname,
				msg,
			)

			if event != nil {
				bus.Events <- *event
			}
		}
	}
}

func convertDockerEvent(
	hostname string,
	msg events.Message,
) *core.Event {

	switch msg.Action {

	case "start":

		return &core.Event{
			ID:        uuid.NewString(),
			Hostname:  hostname,
			Source:    "docker",
			EventType: "CONTAINER_STARTED",
			Severity:  core.INFO,
			Title:     "Container Started",
			Message: fmt.Sprintf(
				"Container %s started",
				msg.Actor.Attributes["name"],
			),
			Timestamp: time.Now(),
		}

	case "stop":

		return &core.Event{
			ID:        uuid.NewString(),
			Hostname:  hostname,
			Source:    "docker",
			EventType: "CONTAINER_STOPPED",
			Severity:  core.WARN,
			Title:     "Container Stopped",
			Message: fmt.Sprintf(
				"Container %s stopped",
				msg.Actor.Attributes["name"],
			),
			Timestamp: time.Now(),
		}

	case "die":

		return &core.Event{
			ID:        uuid.NewString(),
			Hostname:  hostname,
			Source:    "docker",
			EventType: "CONTAINER_DIED",
			Severity:  core.CRITICAL,
			Title:     "Container Died",
			Message: fmt.Sprintf(
				"Container %s exited unexpectedly",
				msg.Actor.Attributes["name"],
			),
			Timestamp: time.Now(),
		}
	}

	return nil
}
