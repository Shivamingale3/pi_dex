package main

import (
	"bufio"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Shivamingale3/pi_dex/internal/config"
	"github.com/Shivamingale3/pi_dex/internal/health"
)

func manageHealthChecks(cfg config.Config, configPath string, scanner *bufio.Scanner) config.Config {
	for {
		fmt.Printf("\n\x1b[1mManage Health Checks\x1b[0m\n\n")
		if len(cfg.HealthChecks) == 0 {
			fmt.Println("  No health checks configured.")
		}
		for i, hc := range cfg.HealthChecks {
			fmt.Printf("  %d. %-20s %s\n", i+1, hc.Name, health.Target(hc))
		}
		fmt.Println()
		fmt.Println("  [A]dd  [E]dit  [D]elete  [T]est now  [B]ack")
		fmt.Print("  Choice: ")
		if !scanner.Scan() {
			return cfg
		}

		switch strings.ToUpper(strings.TrimSpace(scanner.Text())) {
		case "A":
			hc, ok := promptHealthCheck(config.HealthCheckConfig{}, cfg.HealthChecks, scanner)
			if ok {
				cfg.HealthChecks = append(cfg.HealthChecks, hc)
				saveConfig(configPath, cfg)
				fmt.Printf("\x1b[32mAdded '%s'.\x1b[0m\n", hc.Name)
				fmt.Println("\x1b[33mRestart the daemon to apply: sudo systemctl restart pidex\x1b[0m")
			}
		case "E":
			if i, ok := pickHealthCheck(cfg, scanner); ok {
				others := append(append([]config.HealthCheckConfig{}, cfg.HealthChecks[:i]...), cfg.HealthChecks[i+1:]...)
				if hc, ok := promptHealthCheck(cfg.HealthChecks[i], others, scanner); ok {
					cfg.HealthChecks[i] = hc
					saveConfig(configPath, cfg)
					fmt.Printf("\x1b[32mUpdated '%s'.\x1b[0m\n", hc.Name)
					fmt.Println("\x1b[33mRestart the daemon to apply: sudo systemctl restart pidex\x1b[0m")
				}
			}
		case "D":
			if i, ok := pickHealthCheck(cfg, scanner); ok {
				name := cfg.HealthChecks[i].Name
				fmt.Printf("  Delete '%s'? [y/N]: ", name)
				if scanner.Scan() && strings.ToLower(strings.TrimSpace(scanner.Text())) == "y" {
					cfg.HealthChecks = append(cfg.HealthChecks[:i], cfg.HealthChecks[i+1:]...)
					saveConfig(configPath, cfg)
					fmt.Printf("\x1b[32mDeleted '%s'.\x1b[0m\n", name)
				}
			}
		case "T":
			if i, ok := pickHealthCheck(cfg, scanner); ok {
				fmt.Println()
				printHealthResult(cfg.HealthChecks[i])
			}
		case "B", "0", "":
			return cfg
		}
	}
}

func pickHealthCheck(cfg config.Config, scanner *bufio.Scanner) (int, bool) {
	if len(cfg.HealthChecks) == 0 {
		fmt.Println("\x1b[33mNo health checks configured.\x1b[0m")
		return 0, false
	}
	fmt.Printf("  Number [1-%d]: ", len(cfg.HealthChecks))
	if !scanner.Scan() {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil || n < 1 || n > len(cfg.HealthChecks) {
		fmt.Println("\x1b[33mInvalid choice.\x1b[0m")
		return 0, false
	}
	return n - 1, true
}

// promptHealthCheck asks for every field relevant to the check's type, using
// cur as the defaults (blank keeps the current value, "-" clears it). It
// returns false if the result does not validate.
func promptHealthCheck(cur config.HealthCheckConfig, others []config.HealthCheckConfig, scanner *bufio.Scanner) (config.HealthCheckConfig, bool) {
	hc := cur
	fmt.Println("\n  Blank keeps the current value, '-' clears an optional field.")

	hc.Name = promptString(scanner, "Name", hc.Name, false)
	for {
		hc.Type = strings.ToLower(promptString(scanner, "Type ("+strings.Join(config.HealthCheckTypes, "|")+")", hc.Type, false))
		if isHealthType(hc.Type) {
			break
		}
		fmt.Println("\x1b[33m  Unknown type.\x1b[0m")
		if hc.Type == "" {
			return cur, false
		}
		hc.Type = ""
	}
	hc = hc.WithDefaults()

	switch hc.Type {
	case config.HealthHTTP:
		hc.Address = promptString(scanner, "URL (http://host:port/path)", hc.Address, false)
		hc.ExpectStatus = promptInt(scanner, "Expected status", hc.ExpectStatus)
		hc.Headers = promptHeaders(scanner, hc.Headers)
	case config.HealthTCP:
		hc.Address = promptString(scanner, "Address (host:port)", hc.Address, false)
	case config.HealthRedis:
		hc.Address = promptString(scanner, "Address (host:port)", hc.Address, false)
		hc.Username = promptString(scanner, "ACL username (optional)", hc.Username, true)
		hc.Password = promptSecret(scanner, "Password (optional)", hc.Password)
	case config.HealthSystemd:
		hc.Unit = promptString(scanner, "Unit", hc.Unit, false)
	case config.HealthDocker:
		hc.Container = promptString(scanner, "Container name", hc.Container, false)
	case config.HealthCommand:
		hc.Command = promptString(scanner, "Command (run via /bin/sh -c, exit 0 = healthy)", hc.Command, false)
	case config.HealthInterface:
		hc.Interface = promptString(scanner, "Interface", hc.Interface, false)
	}

	if hc.Type != config.HealthSystemd {
		hc.Unit = promptString(scanner, "Systemd unit for diagnosis/logs (optional)", hc.Unit, true)
	}
	if hc.Type != config.HealthDocker {
		hc.Container = promptString(scanner, "Docker container for diagnosis/logs (optional)", hc.Container, true)
	}
	if hc.Type != config.HealthInterface {
		hc.Interface = promptString(scanner, "Interface for diagnosis (optional)", hc.Interface, true)
	}
	hc.Interval = promptInt(scanner, "Interval (seconds)", hc.Interval)
	hc.Timeout = promptInt(scanner, "Timeout (seconds)", hc.Timeout)
	hc.Failures = promptInt(scanner, "Consecutive failures before alert", hc.Failures)

	// Drop fields that belong to other types, so a type change leaves no stale config.
	if hc.Type != config.HealthHTTP {
		hc.ExpectStatus, hc.Headers = 0, nil
	}
	if hc.Type != config.HealthHTTP && hc.Type != config.HealthTCP && hc.Type != config.HealthRedis {
		hc.Address = ""
	}
	if hc.Type != config.HealthRedis {
		hc.Username, hc.Password = "", ""
	}
	if hc.Type != config.HealthCommand {
		hc.Command = ""
	}

	if err := hc.Validate(others); err != nil {
		fmt.Printf("\x1b[31mNot saved: %v\x1b[0m\n", err)
		return cur, false
	}
	return hc, true
}

func isHealthType(t string) bool {
	for _, v := range config.HealthCheckTypes {
		if v == t {
			return true
		}
	}
	return false
}

func promptString(scanner *bufio.Scanner, label, cur string, optional bool) string {
	fmt.Printf("  %s [%s]: ", label, cur)
	if !scanner.Scan() {
		return cur
	}
	v := strings.TrimSpace(scanner.Text())
	switch {
	case v == "":
		return cur
	case v == "-" && optional:
		return ""
	}
	return v
}

func promptSecret(scanner *bufio.Scanner, label, cur string) string {
	fmt.Printf("  %s [%s]: ", label, config.MaskSecret(cur))
	if !scanner.Scan() {
		return cur
	}
	switch v := strings.TrimSpace(scanner.Text()); v {
	case "":
		return cur
	case "-":
		return ""
	default:
		return v
	}
}

func promptInt(scanner *bufio.Scanner, label string, cur int) int {
	fmt.Printf("  %s [%d]: ", label, cur)
	if !scanner.Scan() {
		return cur
	}
	v := strings.TrimSpace(scanner.Text())
	if v == "" {
		return cur
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		fmt.Println("\x1b[33m  Not a number, keeping current value.\x1b[0m")
		return cur
	}
	return n
}

// promptHeaders edits HTTP headers one at a time. Values are secrets.
func promptHeaders(scanner *bufio.Scanner, cur map[string]string) map[string]string {
	h := make(map[string]string, len(cur))
	for k, v := range cur {
		h[k] = v
	}
	for {
		fmt.Printf("  Headers [%s]\n", config.MaskedHeaders(h))
		fmt.Print("  Add 'Name: value', '-Name' to remove, blank when done: ")
		if !scanner.Scan() {
			break
		}
		v := strings.TrimSpace(scanner.Text())
		if v == "" {
			break
		}
		if strings.HasPrefix(v, "-") {
			delete(h, strings.TrimSpace(v[1:]))
			continue
		}
		name, value, ok := strings.Cut(v, ":")
		if !ok || strings.TrimSpace(name) == "" {
			fmt.Println("\x1b[33m  Use 'Name: value'.\x1b[0m")
			continue
		}
		h[strings.TrimSpace(name)] = strings.TrimSpace(value)
	}
	if len(h) == 0 {
		return nil
	}
	return h
}

// printHealthChecks shows the configured checks with secrets masked.
func printHealthChecks(checks []config.HealthCheckConfig) {
	fmt.Println("\nHealth checks:")
	if len(checks) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, hc := range checks {
		fmt.Printf("  %s (%s) every %ds, timeout %ds, alert after %d failures\n",
			hc.Name, health.Target(hc), hc.Interval, hc.Timeout, hc.Failures)
		var extra []string
		add := func(k, v string) {
			if v != "" {
				extra = append(extra, k+"="+v)
			}
		}
		add("unit", hc.Unit)
		add("container", hc.Container)
		add("interface", hc.Interface)
		add("username", hc.Username)
		add("password", config.MaskSecret(hc.Password))
		if hc.Type == config.HealthHTTP {
			add("expect_status", strconv.Itoa(hc.ExpectStatus))
		}
		add("headers", config.MaskedHeaders(hc.Headers))
		if hc.Type == config.HealthCommand {
			add("command", hc.Command)
		}
		sort.Strings(extra)
		if len(extra) > 0 {
			fmt.Printf("    %s\n", strings.Join(extra, "  "))
		}
	}
}
