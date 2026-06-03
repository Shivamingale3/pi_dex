package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/leadows/pi_dex/internal/config"
	"github.com/leadows/pi_dex/internal/core"
	"github.com/leadows/pi_dex/internal/notifier"
)

var defaultCooldowns = map[string]float64{
	core.EventSSHLogin:      core.CooldownSSHLogin,
	core.EventSSHLogout:     core.CooldownSSHLogout,
	core.EventSSHBruteforce: core.CooldownSSHBruteforce,
	core.EventSudoUsed:      core.CooldownSudoUsed,
	core.EventCPUHigh:       core.CooldownCPUHigh,
	core.EventCPURecovered:  core.CooldownCPURecovered,
	core.EventTempWarn:      core.CooldownTempWarn,
	core.EventTempCritical:  core.CooldownTempCritical,
	core.EventDiskWarn:      core.CooldownDiskWarn,
	core.EventDiskCritical:  core.CooldownDiskCritical,
	core.EventRAMHigh:       core.CooldownRAMHigh,
}

func cmdSetup(cfg config.Config) {
	configPath, envPath := resolvePaths()

	cfgDir := filepath.Dir(configPath)
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Run with: sudo pidex setup\n")
		os.Exit(1)
	}
	testFile := filepath.Join(cfgDir, ".write_test")
	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Run with: sudo pidex setup\n")
		os.Exit(1)
	}
	os.Remove(testFile)

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Printf("\n\x1b[1mPiDex Setup Wizard\x1b[0m\n")
		fmt.Printf("  Config: %s\n", configPath)
		token, _ := readEnv(envPath)
		if token != "" || cfg.TelegramToken != "" {
			fmt.Println("  Credentials: set")
		} else {
			fmt.Println("  Credentials: NOT SET")
		}
		fmt.Println()
		fmt.Println("  1. View current config")
		fmt.Println("  2. Set Telegram credentials")
		fmt.Println("  3. Set monitor toggles")
		fmt.Println("  4. Set poller intervals")
		fmt.Println("  5. Set thresholds")
		fmt.Println("  6. Set watch lists")
		fmt.Println("  7. Set cooldowns")
		fmt.Println("  8. Send test notification")
		fmt.Println("  9. Reset to defaults")
		fmt.Println("  0. Save & exit")
		fmt.Print("\nChoice [0-9]: ")

		if !scanner.Scan() {
			break
		}
		raw := strings.TrimSpace(scanner.Text())

		switch raw {
		case "0", "":
			fmt.Println("Exiting. Configuration saved.")
			return
		case "1":
			viewConfig(cfg, envPath)
		case "2":
			cfg = setCredentials(cfg, envPath)
		case "3":
			cfg = setMonitor(cfg, configPath)
		case "4":
			cfg = setIntervals(cfg, configPath)
		case "5":
			cfg = setThresholds(cfg, configPath)
		case "6":
			cfg = setWatchLists(cfg, configPath)
		case "7":
			cfg = setCooldowns(cfg, configPath)
		case "8":
			sendTest(cfg)
		case "9":
			cfg = resetDefaults(cfg, configPath, envPath)
		default:
			fmt.Println("Enter 0-9")
		}
	}
}

func resolvePaths() (configPath, envPath string) {
	for _, p := range []string{"/etc/pidex/config.toml", os.ExpandEnv("~/.config/pidex/config.toml"), "./config/config.toml"} {
		if _, err := os.Stat(p); err == nil {
			configPath = p
			envPath = filepath.Join(filepath.Dir(p), "env")
			return
		}
	}
	if os.Geteuid() == 0 {
		configPath = "/etc/pidex/config.toml"
		envPath = "/etc/pidex/env"
	} else {
		configPath = os.ExpandEnv("~/.config/pidex/config.toml")
		envPath = os.ExpandEnv("~/.config/pidex/env")
	}
	return
}

func readEnv(path string) (token, chatID string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TELEGRAM_BOT_TOKEN=") {
			token = strings.TrimPrefix(line, "TELEGRAM_BOT_TOKEN=")
		} else if strings.HasPrefix(line, "TELEGRAM_CHAT_ID=") {
			chatID = strings.TrimPrefix(line, "TELEGRAM_CHAT_ID=")
		}
	}
	return
}

func writeEnv(path, token, chatID string) error {
	os.MkdirAll(filepath.Dir(path), 0755)
	content := fmt.Sprintf("TELEGRAM_BOT_TOKEN=%s\nTELEGRAM_CHAT_ID=%s\n", token, chatID)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return err
	}
	fmt.Printf("\x1b[32mWrote %s (mode 600)\x1b[0m\n", path)
	return nil
}

func viewConfig(cfg config.Config, envPath string) {
	token, _ := readEnv(envPath)
	fmt.Println("\nCredentials:")
	if token != "" {
		fmt.Println("  bot_token: ***")
		fmt.Println("  chat_id:   ***")
		fmt.Printf("  source:    env file (%s)\n", envPath)
	} else {
		fmt.Printf("  bot_token: %s\n", maskToken(cfg.TelegramToken))
		fmt.Printf("  chat_id:   %s\n", cfg.TelegramChatID)
		fmt.Println("  source:    config.toml")
	}

	fmt.Println("\nMonitor:")
	for _, k := range []string{"ssh", "sudo", "docker", "systemd", "network", "cpu", "ram", "disk", "temperature"} {
		v := fieldBool(cfg, "Monitor"+strings.ToUpper(k[:1])+k[1:])
		fmt.Printf("  %s: ", k)
		if v {
			fmt.Println("on")
		} else {
			fmt.Println("off")
		}
	}

	fmt.Println("\nPoller intervals (seconds):")
	fmt.Printf("  CPU: %d\n", cfg.CPUInterval)
	fmt.Printf("  RAM: %d\n", cfg.RAMInterval)
	fmt.Printf("  Temperature: %d\n", cfg.TempInterval)
	fmt.Printf("  Disk: %d\n", cfg.DiskInterval)

	fmt.Println("\nThresholds (%):")
	fmt.Printf("  CPU warn: %.0f  crit: %.0f\n", cfg.CPUWarn, cfg.CPUCritical)
	fmt.Printf("  RAM warn: %.0f  crit: %.0f\n", cfg.RAMWarn, cfg.RAMCritical)
	fmt.Printf("  Disk warn: %.0f  crit: %.0f\n", cfg.DiskWarn, cfg.DiskCritical)
	fmt.Printf("  Temp warn: %.0f  crit: %.0f\n", cfg.TempWarn, cfg.TempCritical)

	fmt.Println("\nWatch lists:")
	if cfg.ServiceWatch != nil {
		fmt.Printf("  services:   %v\n", cfg.ServiceWatch)
	} else {
		fmt.Println("  services:   (all)")
	}
	if cfg.ContainerWatch != nil {
		fmt.Printf("  containers: %v\n", cfg.ContainerWatch)
	} else {
		fmt.Println("  containers: (all)")
	}

	fmt.Println("\nCooldown overrides (seconds):")
	if len(cfg.CooldownOverrides) > 0 {
		for k, v := range cfg.CooldownOverrides {
			fmt.Printf("  %s: %.0f\n", k, v)
		}
	} else {
		fmt.Println("  (defaults)")
	}
}

func fieldBool(cfg config.Config, name string) bool {
	switch name {
	case "MonitorSSH":
		return cfg.MonitorSSH
	case "MonitorSudo":
		return cfg.MonitorSudo
	case "MonitorDocker":
		return cfg.MonitorDocker
	case "MonitorSystemd":
		return cfg.MonitorSystemd
	case "MonitorNetwork":
		return cfg.MonitorNetwork
	case "MonitorCPU":
		return cfg.MonitorCPU
	case "MonitorRAM":
		return cfg.MonitorRAM
	case "MonitorDisk":
		return cfg.MonitorDisk
	case "MonitorTemperature":
		return cfg.MonitorTemperature
	}
	return false
}

func maskToken(token string) string {
	if len(token) > 8 {
		return token[:4] + "..." + token[len(token)-4:]
	}
	return "***"
}

func setCredentials(cfg config.Config, envPath string) config.Config {
	curToken, curChat := readEnv(envPath)
	token := curToken
	chatID := curChat
	if token == "" {
		token = cfg.TelegramToken
	}
	if chatID == "" {
		chatID = cfg.TelegramChatID
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("\nTelegram credentials (leave blank to keep current):")
	fmt.Printf("  Bot token [%s]: ", maskToken(token))
	if scanner.Scan() {
		if v := strings.TrimSpace(scanner.Text()); v != "" {
			token = v
		}
	}
	fmt.Printf("  Chat ID [%s]: ", maskToken(chatID))
	if scanner.Scan() {
		if v := strings.TrimSpace(scanner.Text()); v != "" {
			chatID = v
		}
	}

	if token == "" || chatID == "" {
		fmt.Println("\x1b[33mBoth token and chat_id are required.\x1b[0m")
		return cfg
	}
	if !strings.Contains(token, ":") {
		fmt.Println("\x1b[33mInvalid bot token format (expected digits:hex).\x1b[0m")
		return cfg
	}
	if _, err := strconv.ParseInt(strings.TrimLeft(chatID, "-"), 10, 64); err != nil {
		fmt.Println("\x1b[33mChat ID must be numeric.\x1b[0m")
		return cfg
	}

	if err := writeEnv(envPath, token, chatID); err != nil {
		fmt.Printf("\x1b[31mFailed to write %s: %v\x1b[0m\n", envPath, err)
		return cfg
	}
	cfg.TelegramToken = token
	cfg.TelegramChatID = chatID
	fmt.Println("\x1b[32mCredentials saved.\x1b[0m")
	return cfg
}

func setMonitor(cfg config.Config, configPath string) config.Config {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("\nMonitor toggles (y/n, blank to keep):")

	keys := []string{"ssh", "sudo", "docker", "systemd", "network", "cpu", "ram", "disk", "temperature"}
	for _, k := range keys {
		current := fieldBool(cfg, "Monitor"+strings.ToUpper(k[:1])+k[1:])
		label := "Y"
		if !current {
			label = "y"
		}
		label2 := "n"
		if current {
			label2 = "N"
		}
		fmt.Printf("  Monitor %s? [%s/%s]: ", k, label, label2)
		if scanner.Scan() {
			v := strings.ToLower(strings.TrimSpace(scanner.Text()))
			if v == "y" {
				setFieldBool(&cfg, "Monitor"+strings.ToUpper(k[:1])+k[1:], true)
			} else if v == "n" {
				setFieldBool(&cfg, "Monitor"+strings.ToUpper(k[:1])+k[1:], false)
			}
		}
	}

	saveConfig(configPath, cfg)
	return cfg
}

func setFieldBool(cfg *config.Config, name string, val bool) {
	switch name {
	case "MonitorSSH":
		cfg.MonitorSSH = val
	case "MonitorSudo":
		cfg.MonitorSudo = val
	case "MonitorDocker":
		cfg.MonitorDocker = val
	case "MonitorSystemd":
		cfg.MonitorSystemd = val
	case "MonitorNetwork":
		cfg.MonitorNetwork = val
	case "MonitorCPU":
		cfg.MonitorCPU = val
	case "MonitorRAM":
		cfg.MonitorRAM = val
	case "MonitorDisk":
		cfg.MonitorDisk = val
	case "MonitorTemperature":
		cfg.MonitorTemperature = val
	}
}

func setIntervals(cfg config.Config, configPath string) config.Config {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("\nPoller intervals in seconds (blank to keep):")

	prompts := []struct {
		label string
		ptr   *int
	}{
		{"CPU", &cfg.CPUInterval},
		{"RAM", &cfg.RAMInterval},
		{"Temperature", &cfg.TempInterval},
		{"Disk", &cfg.DiskInterval},
	}
	for _, p := range prompts {
		fmt.Printf("  %s [%d]: ", p.label, *p.ptr)
		if scanner.Scan() {
			if v := strings.TrimSpace(scanner.Text()); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					*p.ptr = n
				}
			}
		}
	}

	saveConfig(configPath, cfg)
	return cfg
}

func setThresholds(cfg config.Config, configPath string) config.Config {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("\nThresholds in percent (blank to keep):")

	prompts := []struct {
		label string
		ptr   *float64
	}{
		{"CPU warn", &cfg.CPUWarn},
		{"CPU critical", &cfg.CPUCritical},
		{"RAM warn", &cfg.RAMWarn},
		{"RAM critical", &cfg.RAMCritical},
		{"Disk warn", &cfg.DiskWarn},
		{"Disk critical", &cfg.DiskCritical},
		{"Temp warn", &cfg.TempWarn},
		{"Temp critical", &cfg.TempCritical},
	}
	for _, p := range prompts {
		fmt.Printf("  %s [%.0f]: ", p.label, *p.ptr)
		if scanner.Scan() {
			if v := strings.TrimSpace(scanner.Text()); v != "" {
				if n, err := strconv.ParseFloat(v, 64); err == nil {
					*p.ptr = n
				}
			}
		}
	}

	saveConfig(configPath, cfg)
	return cfg
}

func setWatchLists(cfg config.Config, configPath string) config.Config {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("\nWatch lists (comma-separated glob patterns, blank to keep):")

	fmt.Printf("  Services %v: ", cfg.ServiceWatch)
	if scanner.Scan() {
		if v := strings.TrimSpace(scanner.Text()); v != "" {
			var list []string
			for _, item := range strings.Split(v, ",") {
				if item = strings.TrimSpace(item); item != "" {
					list = append(list, item)
				}
			}
			cfg.ServiceWatch = list
		}
	}

	fmt.Printf("  Containers %v: ", cfg.ContainerWatch)
	if scanner.Scan() {
		if v := strings.TrimSpace(scanner.Text()); v != "" {
			var list []string
			for _, item := range strings.Split(v, ",") {
				if item = strings.TrimSpace(item); item != "" {
					list = append(list, item)
				}
			}
			cfg.ContainerWatch = list
		}
	}

	saveConfig(configPath, cfg)
	return cfg
}

func setCooldowns(cfg config.Config, configPath string) config.Config {
	scanner := bufio.NewScanner(os.Stdin)
	overrides := make(map[string]float64)
	if cfg.CooldownOverrides != nil {
		for k, v := range cfg.CooldownOverrides {
			overrides[k] = v
		}
	}

	fmt.Println("\nCooldown overrides in seconds (blank to keep, 0 = no cooldown):")
	for k, def := range defaultCooldowns {
		current, ok := overrides[k]
		if !ok {
			current = def
		}
		fmt.Printf("  %s [%.0f]: ", k, current)
		if scanner.Scan() {
			if v := strings.TrimSpace(scanner.Text()); v != "" {
				if n, err := strconv.ParseFloat(v, 64); err == nil {
					if n == def {
						delete(overrides, k)
					} else {
						overrides[k] = n
					}
				}
			}
		}
	}

	if len(overrides) > 0 {
		cfg.CooldownOverrides = overrides
	} else {
		cfg.CooldownOverrides = nil
	}

	saveConfig(configPath, cfg)
	return cfg
}

func sendTest(cfg config.Config) {
	_, envPath := resolvePaths()
	token, chatID := readEnv(envPath)
	if token == "" {
		token = cfg.TelegramToken
	}
	if chatID == "" {
		chatID = cfg.TelegramChatID
	}
	if token == "" || chatID == "" {
		fmt.Println("\x1b[33mNo Telegram credentials configured. Run option 2 first.\x1b[0m")
		return
	}

	n := notifier.NewTelegramNotifier(token, chatID)
	event := core.Event{
		Source:    core.SourceDaemon,
		EventType: "TEST",
		Severity:  core.SeverityInfo,
		Title:     "Test Notification",
		Message:   "This is a test message from PiDex setup wizard",
	}
	if err := n.Send(event); err != nil {
		fmt.Printf("\x1b[33mFailed: %v\x1b[0m\n", err)
		return
	}
	fmt.Println("\x1b[32mTest notification sent!\x1b[0m")
}

func resetDefaults(cfg config.Config, configPath, envPath string) config.Config {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("\nReset to factory defaults? This will erase your configuration.")
	fmt.Print("Type 'reset' to confirm: ")
	if !scanner.Scan() {
		return cfg
	}
	if strings.TrimSpace(strings.ToLower(scanner.Text())) != "reset" {
		fmt.Println("\x1b[33mReset cancelled.\x1b[0m")
		return cfg
	}

	cfg = config.DefaultConfig()
	saveConfig(configPath, cfg)
	os.Remove(envPath)
	fmt.Println("\x1b[32mConfiguration reset to defaults.\x1b[0m")
	return cfg
}

func saveConfig(path string, cfg config.Config) {
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := config.SaveConfig(path, cfg); err != nil {
		fmt.Printf("\x1b[31mFailed to write %s: %v\x1b[0m\n", path, err)
		return
	}
	fmt.Printf("\x1b[32mWrote %s\x1b[0m\n", path)
}
