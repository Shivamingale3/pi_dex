package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/leadows/pi_dex/internal/core"
)

const githubReleases = "https://api.github.com/repos/Shivamingale3/pi_dex/releases/latest"

type releaseResponse struct {
	TagName string `json:"tag_name"`
}

func cmdUpdate() {
	fmt.Print("Checking for updates... ")

	resp, err := http.Get(githubReleases)
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

	latest := "v" + core.Version
	if rel.TagName == latest {
		fmt.Printf("up to date (%s)\n", latest)
		return
	}

	fmt.Printf("%s available (current: %s)\n", rel.TagName, latest)

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
		if err := downloadBinary(url, b.path); err != nil {
			fmt.Fprintf(os.Stderr, "Failed: %v\n", err)
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

func downloadBinary(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "pidex-update-")
	if err != nil {
		return err
	}
	path := tmp.Name()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(path)
		return err
	}
	tmp.Close()

	if err := os.Chmod(path, 0755); err != nil {
		os.Remove(path)
		return err
	}

	if err := os.Rename(path, dest); err != nil {
		os.Remove(path)
		return err
	}

	return nil
}
