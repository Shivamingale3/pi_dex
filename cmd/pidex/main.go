package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/leadows/pi_dex/internal/config"
	"github.com/leadows/pi_dex/internal/core"
	"github.com/leadows/pi_dex/internal/notifier"
	"github.com/leadows/pi_dex/internal/parser"
	"github.com/leadows/pi_dex/internal/poller"
	"github.com/leadows/pi_dex/internal/source"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	switch os.Args[1] {
	case "run":
		cmdRun()
	case "version":
		fmt.Printf("PiDex v%s\n", core.Version)
	case "setup":
		requireRoot()
		cfg := config.LoadConfig("")
		cmdSetup(cfg)
	case "test":
		if len(os.Args) < 3 || !hasDryRun(os.Args) {
			requireRoot()
		}
		cmdTest(os.Args[1:])
	case "uninstall":
		requireRoot()
		cmdUninstall()
	case "update":
		requireRoot()
		cmdUpdate()
	case "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		usage()
	}
}

func requireRoot() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "pidex: this command requires root.")
		fmt.Fprintln(os.Stderr, "Run with: sudo pidex", strings.Join(os.Args[1:], " "))
		os.Exit(1)
	}
}

func hasDryRun(args []string) bool {
	for _, a := range args {
		if a == "--dry-run" {
			return true
		}
	}
	return false
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: pidex <command>\n")
	fmt.Fprintf(os.Stderr, "\nCommands:\n")
	fmt.Fprintf(os.Stderr, "  run        Start the daemon\n")
	fmt.Fprintf(os.Stderr, "  setup      Interactive configuration wizard\n")
	fmt.Fprintf(os.Stderr, "  test       Send a test notification\n")
	fmt.Fprintf(os.Stderr, "  uninstall  Remove PiDex from the system\n")
	fmt.Fprintf(os.Stderr, "  update     Update PiDex to the latest release\n")
	fmt.Fprintf(os.Stderr, "  version    Show version\n")
	fmt.Fprintf(os.Stderr, "  help       Show this help\n")
	os.Exit(1)
}

func cmdRun() {
	cfg := config.LoadConfig("")

	if cfg.TelegramToken == "" || cfg.TelegramChatID == "" {
		log.Fatal("Telegram bot_token and chat_id required (config or TELEGRAM_BOT_TOKEN/TELEGRAM_CHAT_ID env)")
	}

	bus := core.NewEventBus(64)
	tg := notifier.NewTelegramNotifier(cfg.TelegramToken, cfg.TelegramChatID)
	cd := core.NewCooldownManager(cfg.CooldownOverrides)
	dd := core.NewDedupManager()
	disp := core.NewDispatcher(bus, tg, cd, dd)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go disp.Run(ctx)

	var sources []source.Source

	if cfg.MonitorSSH || cfg.MonitorSudo || cfg.MonitorSystemd {
		journal := source.NewJournalSource(bus)
		if cfg.MonitorSSH {
			journal.Register(parser.ParseSSH)
		}
		if cfg.MonitorSudo {
			journal.Register(parser.ParseSudo)
		}
		if cfg.MonitorSystemd {
			journal.Register(parser.MakeSystemdParser(cfg.ServiceWatch))
		}
		sources = append(sources, journal)
	}

	if cfg.MonitorDocker {
		sources = append(sources, source.NewDockerSource(bus, cfg.ContainerWatch))
	}

	if cfg.MonitorNetwork {
		sources = append(sources, source.NewNetworkSource(bus))
	}

	sourceCount := len(sources)

	for _, s := range sources {
		go func(s source.Source) {
			if err := s.Run(ctx); err != nil {
				log.Printf("Source error: %v", err)
			}
		}(s)
	}

	pollerCount := startPollers(ctx, bus, cfg)

	bus.Publish(core.Event{
		Source:    core.SourceDaemon,
		EventType: core.EventDaemonStart,
		Severity:  core.SeverityInfo,
		Title:     "PiDex Started",
		Message:   fmt.Sprintf("PiDex v%s started (%d sources, %d pollers)", core.Version, sourceCount, pollerCount),
		Timestamp: time.Now(),
	})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	bus.Publish(core.Event{
		Source:    core.SourceShutdown,
		EventType: core.EventShutdownStarted,
		Severity:  core.SeverityWarn,
		Title:     "Shutdown Initiated",
		Message:   "PiDex daemon is shutting down",
		Timestamp: time.Now(),
	})

	time.Sleep(200 * time.Millisecond)
	disp.Drain()

	cancel()
	log.Println("PiDex stopped")
}

func startPollers(ctx context.Context, bus *core.EventBus, cfg config.Config) int {
	if cfg.MonitorCPU {
		p := poller.NewCpuPoller(bus, cfg.CPUInterval, cfg.CPUWarn, cfg.CPUCritical)
		go p.Run(ctx)
	}

	if cfg.MonitorRAM {
		p := poller.NewRamPoller(bus, cfg.RAMInterval, cfg.RAMWarn, cfg.RAMCritical)
		go p.Run(ctx)
	}

	if cfg.MonitorDisk {
		p := poller.NewDiskPoller(bus, cfg.DiskInterval, cfg.DiskWarn, cfg.DiskCritical, "/")
		go p.Run(ctx)
	}

	if cfg.MonitorTemperature {
		p := poller.NewTemperaturePoller(bus, cfg.TempInterval, cfg.TempWarn, cfg.TempCritical)
		go p.Run(ctx)
	}

	count := 0
	if cfg.MonitorCPU {
		count++
	}
	if cfg.MonitorRAM {
		count++
	}
	if cfg.MonitorDisk {
		count++
	}
	if cfg.MonitorTemperature {
		count++
	}
	return count
}
