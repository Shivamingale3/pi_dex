package main

import (
	"fmt"
	"os"
	"os/exec"
)

func cmdUninstall() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "pidex uninstall must be run as root (try: sudo pidex uninstall)")
		os.Exit(1)
	}

	for _, svc := range []string{"pidex", "pidex-shutdown"} {
		exec.Command("systemctl", "disable", "--now", svc).Run()
	}

	os.Remove("/usr/local/bin/pidex")
	os.Remove("/usr/local/bin/pidex-shutdown")
	os.Remove("/etc/systemd/system/pidex.service")
	os.Remove("/etc/systemd/system/pidex-shutdown.service")

	var answer string
	fmt.Print("Remove /etc/pidex? [y/N]: ")
	fmt.Scanln(&answer)
	if answer == "y" || answer == "Y" {
		os.RemoveAll("/etc/pidex")
		fmt.Println("Removed /etc/pidex")
	}

	fmt.Print("Remove pidex system user? [y/N]: ")
	fmt.Scanln(&answer)
	if answer == "y" || answer == "Y" {
		exec.Command("userdel", "pidex").Run()
		fmt.Println("Removed user: pidex")
	}

	exec.Command("systemctl", "daemon-reload").Run()
	fmt.Println("PiDex uninstalled.")
}
