package source

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"os/exec"

	"github.com/leadows/pi_dex/internal/core"
)

type JournalParser func(map[string]any) *core.Event

type JournalSource struct {
	bus     *core.EventBus
	parsers []JournalParser
}

func NewJournalSource(bus *core.EventBus) *JournalSource {
	return &JournalSource{bus: bus}
}

func (s *JournalSource) Register(p JournalParser) {
	s.parsers = append(s.parsers, p)
}

func (s *JournalSource) Run(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "journalctl", "-f", "--output=json", "--since=now")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("journald: stdout pipe: %v", err)
		return err
	}

	if err := cmd.Start(); err != nil {
		log.Printf("journald: start: %v", err)
		return err
	}
	defer cmd.Wait()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		if !isRelevant(entry) {
			continue
		}

		for _, parser := range s.parsers {
			event := parser(entry)
			if event != nil {
				s.bus.Publish(*event)
			}
		}

		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}

	return scanner.Err()
}

var relevantComms = map[string]bool{
	"sshd":         true,
	"sshd-session": true,
	"sudo":         true,
	"systemd":      true,
}

func isRelevant(entry map[string]any) bool {
	comm, _ := entry["_COMM"].(string)
	if relevantComms[comm] {
		return true
	}
	ident, _ := entry["SYSLOG_IDENTIFIER"].(string)
	if relevantComms[ident] {
		return true
	}
	unit, _ := entry["_SYSTEMD_UNIT"].(string)
	if len(unit) > 8 && unit[len(unit)-8:] == ".service" {
		return true
	}
	return false
}
