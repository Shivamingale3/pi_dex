package main

import (
	"PI_DEX/internal/cli"
	"PI_DEX/internal/core"
	"PI_DEX/internal/notifier"
	"PI_DEX/internal/pollers"
	"PI_DEX/internal/sources"
	"PI_DEX/internal/testevents"
	"github.com/joho/godotenv"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("unable to load .env file")
	}

	token := os.Getenv("PIDEX_TELEGRAM_TOKEN")
	chatID := os.Getenv("PIDEX_TELEGRAM_CHAT_ID")

	if token == "" {
		log.Fatal("PIDEX_TELEGRAM_TOKEN not set")
	}

	if chatID == "" {
		log.Fatal("PIDEX_TELEGRAM_CHAT_ID not set")
	}

	bus := core.NewEventBus(100)

	dockerSource := sources.NewDockerSource()

	go func() {

		err := dockerSource.Start(bus)

		if err != nil {
			log.Fatal(err)
		}
	}()

	// sshSource := sources.NewSSHSource()

	// go func() {

	// 	err := sshSource.Start(bus)

	// 	if err != nil {
	// 		log.Printf(
	// 			"SSH source failed: %v",
	// 			err,
	// 		)
	// 	}
	// }()

	tg := notifier.NewTelegramNotifier(
		token,
		chatID,
	)

	dispatcher := &core.Dispatcher{
		Notifier: tg,
		Dedup:    core.NewDeduplicator(),
		Cooldown: core.NewCooldownManager(),
	}

	go dispatcher.Start(bus)

	cpuPoller := pollers.NewCPUPoller()
	ramPoller := pollers.NewRAMPoller()
	diskPoller := pollers.NewDiskPoller()

	go cpuPoller.Start(bus)
	go ramPoller.Start(bus)
	go diskPoller.Start(bus)

	command := cli.Parse()

	if command.Action == "test" {

		event, ok := testevents.Get(command.Event)

		if !ok {
			log.Fatal("unknown event")
		}

		if command.DryRun {

			log.Printf(
				"[DRY RUN]\n%s\n%s",
				event.Title,
				event.Message,
			)

			return
		}

		bus.Events <- event

		time.Sleep(2 * time.Second)

		return
	}

	sigChan := make(chan os.Signal, 1)

	signal.Notify(
		sigChan,
		os.Interrupt,
		syscall.SIGTERM,
	)

	<-sigChan

	log.Println("PiDex shutting down...")
}
