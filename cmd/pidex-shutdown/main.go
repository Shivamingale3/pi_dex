package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Shivamingale3/pi_dex/internal/config"
	"github.com/Shivamingale3/pi_dex/internal/core"
	"github.com/Shivamingale3/pi_dex/internal/notifier"
)

func main() {
	cfg := config.LoadConfig("")

	if cfg.TelegramToken == "" || cfg.TelegramChatID == "" {
		fmt.Fprintln(os.Stderr, "pidex-shutdown: no Telegram credentials configured")
		os.Exit(1)
	}

	n := notifier.NewTelegramNotifier(cfg.TelegramToken, cfg.TelegramChatID)
	event := core.Event{
		Source:    core.SourceShutdown,
		EventType: core.EventShutdownStarted,
		Severity:  core.SeverityWarn,
		Title:     "System Shutdown",
		Message:   "Server is powering off",
		Timestamp: time.Now(),
	}

	if err := n.Send(event); err != nil {
		fmt.Fprintf(os.Stderr, "pidex-shutdown: failed: %v\n", err)
		os.Exit(1)
	}
	log.Println("pidex-shutdown: notification sent")
}
