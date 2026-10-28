package main

import (
	"os"
	"strings"
	"testing"
)

func TestEmbeddedUnitsMatchDeployFiles(t *testing.T) {
	for file, embedded := range map[string]string{
		"../../deploy/pidex.service":          pidexServiceUnit,
		"../../deploy/pidex-shutdown.service": pidexShutdownServiceUnit,
	} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != embedded {
			t.Errorf("%s differs from the unit embedded in units.go", file)
		}
	}
}

func TestServiceUnitHasNoNetworkOnlineDependency(t *testing.T) {
	for _, bad := range []string{"network-online.target", "tailscaled.service", "docker.service"} {
		for _, line := range strings.Split(pidexServiceUnit, "\n") {
			if strings.HasPrefix(line, "#") {
				continue
			}
			if strings.Contains(line, bad) {
				t.Errorf("pidex.service depends on %s: %q", bad, line)
			}
		}
	}
	if !strings.Contains(pidexServiceUnit, "\nAfter=network.target\n") {
		t.Error("pidex.service should be ordered After=network.target")
	}
}
