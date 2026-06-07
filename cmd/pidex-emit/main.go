package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	service := flag.String("service", "", "systemd service name")
	message := flag.String("message", "", "log message to write to journald")
	flag.Parse()

	if *service == "" || *message == "" {
		fmt.Fprintln(os.Stderr, "Usage: pidex-emit --service <name> --message <text>")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Emits a journald entry to test PiDex custom service monitoring.")
		os.Exit(1)
	}

	cmd := exec.Command("logger", "-t", *service, *message)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write to journald: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Emitted to journald [%s]: %s\n", *service, *message)
}
