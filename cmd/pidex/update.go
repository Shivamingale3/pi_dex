package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/leadows/pi_dex/internal/core"
)

const githubAPI = "https://api.github.com/repos/Shivamingale3/pi_dex/releases/latest"

type releaseResponse struct {
	TagName string `json:"tag_name"`
}

func cmdUpdate() {
	fmt.Print("Checking for updates... ")

	resp, err := http.Get(githubAPI)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nFailed to check for updates: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "\nGitHub API returned %d\n", resp.StatusCode)
		os.Exit(1)
	}

	var rel releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		fmt.Fprintf(os.Stderr, "\nFailed to parse release info: %v\n", err)
		os.Exit(1)
	}

	latest := strings.TrimPrefix(rel.TagName, "v")

	if !isNewer(latest, core.Version) {
		fmt.Printf("up to date (v%s)\n", core.Version)
		return
	}

	fmt.Printf("v%s available (current: v%s)\n", latest, core.Version)

	arch := runtime.GOARCH
	if arch != "amd64" && arch != "arm64" {
		fmt.Fprintf(os.Stderr, "Unsupported architecture: %s\n", arch)
		os.Exit(1)
	}

	binaries := []struct {
		name string
		path string
	}{
		{"pidex", "/usr/local/bin/pidex"},
		{"pidex-shutdown", "/usr/local/bin/pidex-shutdown"},
	}

	for _, b := range binaries {
		filename := fmt.Sprintf("%s-%s-linux-%s", b.name, rel.TagName, arch)
		url := fmt.Sprintf("https://github.com/Shivamingale3/pi_dex/releases/download/%s/%s", rel.TagName, filename)

		fmt.Printf("Downloading %s...\n", filename)
		if err := downloadFile(url, b.path); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to download %s: %v\n", b.name, err)
			os.Exit(1)
		}
	}

	fmt.Print("Restarting pidex service... ")
	if err := exec.Command("systemctl", "restart", "pidex").Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\nRestart failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("done")

	fmt.Printf("\nUpdated to %s\n", rel.TagName)
}

func isNewer(a, b string) bool {
	aParts := parseVersion(a)
	bParts := parseVersion(b)

	for i := range 3 {
		if aParts[i] > bParts[i] {
			return true
		}
		if aParts[i] < bParts[i] {
			return false
		}
	}
	return false
}

func parseVersion(v string) [3]int {
	parts := strings.Split(v, ".")
	var nums [3]int
	for i, p := range parts {
		if i >= 3 {
			break
		}
		n, _ := strconv.Atoi(p)
		nums[i] = n
	}
	return nums
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "pidex-update-")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	tmp.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, dest); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}
