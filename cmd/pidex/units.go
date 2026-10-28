package main

import (
	"fmt"
	"os"
	"os/exec"
)

// Unit files installed by `pidex update`. They must match deploy/*.service
// (enforced by units_test.go).
const pidexServiceUnit = `[Unit]
Description=PiDex - Home Server Watchman
Documentation=https://github.com/Shivamingale3/pi_dex
# No dependency on network-online.target, tailscaled.service or docker.service:
# PiDex must start at boot even when the network is not ready. Notifications
# that cannot be delivered yet are queued in /var/lib/pidex.
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/pidex run
Restart=on-failure
RestartSec=5
User=pidex
Group=pidex
RuntimeDirectory=pidex
StateDirectory=pidex
ConfigurationDirectory=pidex
StandardOutput=journal
StandardError=journal
EnvironmentFile=-/etc/pidex/env

[Install]
WantedBy=multi-user.target
`

const pidexShutdownServiceUnit = `[Unit]
Description=PiDex Shutdown Notification
Documentation=https://github.com/Shivamingale3/pi_dex
DefaultDependencies=no
Before=shutdown.target reboot.target halt.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/pidex-shutdown
RemainAfterExit=true
User=pidex
Group=pidex
StandardOutput=journal
StandardError=journal
EnvironmentFile=-/etc/pidex/env

[Install]
WantedBy=shutdown.target reboot.target halt.target
`

// installUnits writes the systemd units and reloads systemd.
func installUnits() error {
	units := map[string]string{
		"/etc/systemd/system/pidex.service":          pidexServiceUnit,
		"/etc/systemd/system/pidex-shutdown.service": pidexShutdownServiceUnit,
	}
	for path, content := range units {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %v: %s", err, out)
	}
	return nil
}
