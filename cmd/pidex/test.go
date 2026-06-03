package main

import (
	"fmt"
	"os"

	"github.com/leadows/pi_dex/internal/config"
	"github.com/leadows/pi_dex/internal/core"
	"github.com/leadows/pi_dex/internal/notifier"
)

func cmdTest(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: pidex test <event> [--dry-run]")
		fmt.Fprintln(os.Stderr, "Events: ssh-login, ssh-fail, sudo-used, docker-down, reboot")
		os.Exit(1)
	}

	eventName := args[1]
	dryRun := len(args) > 2 && args[2] == "--dry-run"

	events := map[string]core.Event{
		"ssh-login": {
			Source:    core.SourceSSH,
			EventType: core.EventSSHLogin,
			Severity:  core.SeverityInfo,
			Title:     "SSH Login",
			Message:   "shiv logged in from 192.168.1.100",
		},
		"ssh-fail": {
			Source:    core.SourceSSH,
			EventType: core.EventSSHBruteforce,
			Severity:  core.SeverityWarn,
			Title:     "SSH Brute Force",
			Message:   "5 failed attempts from 10.0.0.50 in 30 seconds",
		},
		"sudo-used": {
			Source:    core.SourcSudo,
			EventType: core.EventSudoUsed,
			Severity:  core.SeverityInfo,
			Title:     "Sudo Used",
			Message:   "shiv ran sudo apt update",
		},
		"docker-down": {
			Source:    core.SourceDocker,
			EventType: core.EventContainerDied,
			Severity:  core.SeverityCritical,
			Title:     "Container Died",
			Message:   "nginx container 'web-prod' exited with code 1",
		},
		"reboot": {
			Source:    core.SourceShutdown,
			EventType: core.EventRebootStarted,
			Severity:  core.SeverityWarn,
			Title:     "Reboot Initiated",
			Message:   "System rebooting by shiv",
		},
	}

	event, ok := events[eventName]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown event: %s\n", eventName)
		fmt.Fprintf(os.Stderr, "Available: ssh-login, ssh-fail, sudo-used, docker-down, reboot\n")
		os.Exit(1)
	}

	if dryRun {
		fmt.Printf("[DRY RUN] Would send: %s\n", event.Title)
		fmt.Printf("  Source: %s\n", event.Source)
		fmt.Printf("  Type: %s\n", event.EventType)
		fmt.Printf("  Severity: %s\n", event.Severity)
		fmt.Printf("  Message: %s\n", event.Message)
		return
	}

	cfg := config.LoadConfig("")
	if cfg.TelegramToken == "" || cfg.TelegramChatID == "" {
		fmt.Fprintln(os.Stderr, "Telegram credentials not configured")
		os.Exit(1)
	}

	n := notifier.NewTelegramNotifier(cfg.TelegramToken, cfg.TelegramChatID)
	if err := n.Send(event); err != nil {
		fmt.Fprintf(os.Stderr, "Failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Sent.")
}
