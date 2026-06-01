package sources

import (
	// "log"
	// "os"
	"regexp"
	"time"

	// "github.com/coreos/go-systemd/v22/sdjournal"
	"github.com/google/uuid"

	"PI_DEX/internal/core"
)

type SSHSource struct {
}

func NewSSHSource() *SSHSource {
	return &SSHSource{}
}

var (
	acceptedPassword = regexp.MustCompile(
		`Accepted (?:password|publickey) for (\S+) from ([0-9a-fA-F\.:]+)`,
	)

	failedPassword = regexp.MustCompile(
		`Failed password for(?: invalid user)? (\S+) from ([0-9a-fA-F\.:]+)`,
	)

	sudoCommand = regexp.MustCompile(
		`sudo: +(\S+) .*COMMAND=(.*)`,
	)
)

// func (s *SSHSource) Start(bus *core.EventBus) error {

// 	hostname, _ := os.Hostname()

// 	journal, err := sdjournal.NewJournal()

// 	if err != nil {
// 		return err
// 	}

// 	defer journal.Close()

// 	err = journal.SeekTail()

// 	if err != nil {
// 		return err
// 	}

// 	_, _ = journal.Next()

// 	log.Println("SSH source started")

// 	for {

// 		n, err := journal.Wait(
// 			time.Second * 5,
// 		)

// 		if err != nil {
// 			log.Printf(
// 				"journal wait error: %v",
// 				err,
// 			)
// 			continue
// 		}

// 		if n == sdjournal.SD_JOURNAL_NOP {
// 			continue
// 		}

// 		for {

// 			count, err := journal.Next()

// 			if err != nil {
// 				break
// 			}

// 			if count == 0 {
// 				break
// 			}

// 			entry, err := journal.GetEntry()

// 			if err != nil {
// 				continue
// 			}

// 			message := entry.Fields["MESSAGE"]

// 			if event := parseSSHLogin(
// 				hostname,
// 				message,
// 			); event != nil {

// 				bus.Events <- *event
// 			}

// 			if event := parseSSHFailure(
// 				hostname,
// 				message,
// 			); event != nil {

// 				bus.Events <- *event
// 			}

// 			if event := parseSudo(
// 				hostname,
// 				message,
// 			); event != nil {

// 				bus.Events <- *event
// 			}
// 		}
// 	}
// }

func parseSSHLogin(
	hostname string,
	message string,
) *core.Event {

	match := acceptedPassword.FindStringSubmatch(
		message,
	)

	if len(match) != 3 {
		return nil
	}

	user := match[1]
	ip := match[2]

	return &core.Event{
		ID:        uuid.NewString(),
		Hostname:  hostname,
		Source:    "ssh",
		EventType: "SSH_LOGIN",
		Severity:  core.INFO,
		Title:     "SSH Login",
		Message: "User " + user +
			" logged in from " + ip,
		Timestamp: time.Now(),
	}
}

func parseSSHFailure(
	hostname string,
	message string,
) *core.Event {

	match := failedPassword.FindStringSubmatch(
		message,
	)

	if len(match) != 3 {
		return nil
	}

	user := match[1]
	ip := match[2]

	return &core.Event{
		ID:        uuid.NewString(),
		Hostname:  hostname,
		Source:    "ssh",
		EventType: "SSH_BRUTEFORCE",
		Severity:  core.WARN,
		Title:     "Failed SSH Login",
		Message: "User " + user +
			" failed login from " + ip,
		Timestamp: time.Now(),
	}
}

func parseSudo(
	hostname string,
	message string,
) *core.Event {

	match := sudoCommand.FindStringSubmatch(
		message,
	)

	if len(match) != 3 {
		return nil
	}

	user := match[1]
	command := match[2]

	return &core.Event{
		ID:        uuid.NewString(),
		Hostname:  hostname,
		Source:    "sudo",
		EventType: "SUDO_USED",
		Severity:  core.INFO,
		Title:     "Sudo Command Executed",
		Message: "User " + user +
			" executed: " + command,
		Timestamp: time.Now(),
	}
}