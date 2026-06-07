package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/Shivamingale3/pi_dex/internal/config"
	"github.com/Shivamingale3/pi_dex/internal/core"
	"github.com/Shivamingale3/pi_dex/internal/notifier"
)

func cmdTest(args []string) {
	if args[1] == "--emit" {
		cmdTestEmit(args)
		return
	}

	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: pidex test <event> [--dry-run]")
		fmt.Fprintln(os.Stderr, "       pidex test --emit --service <name> --event <name> [--message <text>] [--dry-run]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Built-in events: ssh-login, ssh-fail, sudo-used, docker-down, reboot")
		fmt.Fprintln(os.Stderr, "Use --emit to test custom service events via journald")
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

func cmdTestEmit(args []string) {
	service := ""
	eventName := ""
	dryRun := false
	message := ""

	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--service":
			if i+1 < len(args) {
				service = args[i+1]
				i++
			}
		case "--event":
			if i+1 < len(args) {
				eventName = args[i+1]
				i++
			}
		case "--message":
			if i+1 < len(args) {
				message = args[i+1]
				i++
			}
		case "--dry-run":
			dryRun = true
		}
	}

	if service == "" || eventName == "" {
		fmt.Fprintln(os.Stderr, "Usage: pidex test --emit --service <name> --event <name> [--message <text>] [--dry-run]")
		os.Exit(1)
	}

	cfg := config.LoadConfig("")
	svc := findCustomService(cfg, service)
	if svc == nil {
		svc = findCustomServiceInStaging(service)
	}
	if svc == nil {
		fmt.Fprintf(os.Stderr, "Service '%s' not found in registered services or staging directory\n", service)
		os.Exit(1)
	}

	var evt *config.CustomEventConfig
	for i := range svc.Events {
		if svc.Events[i].Name == eventName {
			evt = &svc.Events[i]
			break
		}
	}
	if evt == nil {
		fmt.Fprintf(os.Stderr, "Event '%s' not found in service '%s'\n", eventName, service)
		fmt.Fprintf(os.Stderr, "Available events:\n")
		for _, e := range svc.Events {
			fmt.Fprintf(os.Stderr, "  %s\n", e.Name)
		}
		os.Exit(1)
	}

	var emitMsg string
	if message != "" {
		emitMsg = message
	} else {
		emitMsg = generateMatchMessage(evt.Pattern)
		if emitMsg == "" {
			fmt.Fprintf(os.Stderr, "Cannot auto-generate message from pattern '%s'.\n", evt.Pattern)
			fmt.Fprintf(os.Stderr, "Pattern has regex syntax — provide --message with a matching string.\n")
			os.Exit(1)
		}
	}

	if dryRun {
		fmt.Printf("[DRY RUN] Would emit to journald: logger -t %s %q\n", service, emitMsg)
		fmt.Printf("  Service: %s\n", service)
		fmt.Printf("  Event:   %s\n", eventName)
		fmt.Printf("  Pattern: %s\n", evt.Pattern)
		return
	}

	cmd := exec.Command("logger", "-t", service, emitMsg)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write to journald: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Emitted to journald [%s]: %s\n", service, emitMsg)
	fmt.Println("Make sure the PiDex daemon is running to receive the notification.")
}

func findCustomService(cfg config.Config, name string) *config.CustomServiceConfig {
	for i := range cfg.CustomServices {
		if cfg.CustomServices[i].Name == name {
			return &cfg.CustomServices[i]
		}
	}
	return nil
}

func findCustomServiceInStaging(name string) *config.CustomServiceConfig {
	dirs := []string{
		"/etc/pidex/custom.d",
		filepath.Join(os.Getenv("HOME"), ".config/pidex/custom.d"),
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".conf" {
				continue
			}
			svc := parseCustomServiceFile(filepath.Join(dir, e.Name()))
			if svc.Name == name {
				return &svc
			}
		}
	}
	return nil
}

func generateMatchMessage(pattern string) string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return ""
	}
	prefix, complete := re.LiteralPrefix()
	if complete && prefix != "" {
		return prefix
	}
	return ""
}
