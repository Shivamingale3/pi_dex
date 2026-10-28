package config

import (
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
)

const (
	HealthHTTP      = "http"
	HealthTCP       = "tcp"
	HealthRedis     = "redis"
	HealthSystemd   = "systemd"
	HealthDocker    = "docker"
	HealthCommand   = "command"
	HealthInterface = "interface"
)

// HealthCheckTypes lists the supported check types in display order.
var HealthCheckTypes = []string{HealthHTTP, HealthTCP, HealthRedis, HealthSystemd, HealthDocker, HealthCommand, HealthInterface}

const (
	DefaultHealthInterval     = 30
	DefaultHealthTimeout      = 5
	DefaultHealthFailures     = 3
	DefaultHealthExpectStatus = 200
	MinHealthInterval         = 5
)

type HealthCheckConfig struct {
	Name         string
	Type         string
	Address      string // http: full URL; tcp/redis: host:port
	Unit         string // optional systemd unit, for diagnosis and logs
	Container    string // optional docker container, for diagnosis and logs
	Interface    string // optional network interface, for diagnosis
	Interval     int    // seconds
	Timeout      int    // seconds
	Failures     int    // consecutive failures before alerting
	Password     string // redis AUTH
	Username     string // redis ACL user
	ExpectStatus int    // http
	Headers      map[string]string
	Command      string // command: run via /bin/sh -c, exit 0 = healthy
}

// WithDefaults fills unset numeric fields with their defaults.
func (hc HealthCheckConfig) WithDefaults() HealthCheckConfig {
	if hc.Interval <= 0 {
		hc.Interval = DefaultHealthInterval
	}
	if hc.Timeout <= 0 {
		hc.Timeout = DefaultHealthTimeout
	}
	if hc.Failures <= 0 {
		hc.Failures = DefaultHealthFailures
	}
	if hc.Type == HealthHTTP && hc.ExpectStatus == 0 {
		hc.ExpectStatus = DefaultHealthExpectStatus
	}
	return hc
}

// Validate checks a single health check. others are the remaining configured
// checks, used to enforce unique names.
func (hc HealthCheckConfig) Validate(others []HealthCheckConfig) error {
	if strings.TrimSpace(hc.Name) == "" {
		return fmt.Errorf("name is required")
	}
	for _, o := range others {
		if o.Name == hc.Name {
			return fmt.Errorf("a health check named %q already exists", hc.Name)
		}
	}
	if hc.Interval < MinHealthInterval {
		return fmt.Errorf("interval must be at least %d seconds", MinHealthInterval)
	}
	if hc.Timeout < 1 {
		return fmt.Errorf("timeout must be at least 1 second")
	}
	if hc.Failures < 1 {
		return fmt.Errorf("failures must be at least 1")
	}

	switch hc.Type {
	case HealthHTTP:
		u, err := url.Parse(hc.Address)
		if hc.Address == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("http: address must be a full http:// or https:// URL")
		}
		if hc.ExpectStatus < 100 || hc.ExpectStatus > 599 {
			return fmt.Errorf("http: expect_status must be an HTTP status code")
		}
	case HealthTCP, HealthRedis:
		if _, port, err := net.SplitHostPort(hc.Address); hc.Address == "" || err != nil || port == "" {
			return fmt.Errorf("%s: address must be host:port", hc.Type)
		}
	case HealthSystemd:
		if hc.Unit == "" {
			return fmt.Errorf("systemd: unit is required")
		}
	case HealthDocker:
		if hc.Container == "" {
			return fmt.Errorf("docker: container is required")
		}
	case HealthCommand:
		if strings.TrimSpace(hc.Command) == "" {
			return fmt.Errorf("command: command is required")
		}
	case HealthInterface:
		if hc.Interface == "" {
			return fmt.Errorf("interface: interface is required")
		}
	case "":
		return fmt.Errorf("type is required (%s)", strings.Join(HealthCheckTypes, ", "))
	default:
		return fmt.Errorf("unknown type %q (want %s)", hc.Type, strings.Join(HealthCheckTypes, ", "))
	}
	return nil
}

// ValidateHealthChecks validates every check, including name uniqueness.
func ValidateHealthChecks(checks []HealthCheckConfig) error {
	for i, hc := range checks {
		if err := hc.Validate(checks[:i]); err != nil {
			if hc.Name != "" {
				return fmt.Errorf("health check %q: %w", hc.Name, err)
			}
			return fmt.Errorf("health check %d: %w", i+1, err)
		}
	}
	return nil
}

// UnitName returns the systemd unit name with a ".service" suffix if the
// configured value has no unit type suffix.
func UnitName(unit string) string {
	if unit == "" || strings.Contains(unit, ".") {
		return unit
	}
	return unit + ".service"
}

// MaskSecret hides a secret for display.
func MaskSecret(s string) string {
	if s == "" {
		return ""
	}
	return "***"
}

// MaskedHeaders renders headers as "Name: ***" pairs, sorted by name.
func MaskedHeaders(h map[string]string) string {
	if len(h) == 0 {
		return ""
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + ": " + MaskSecret(h[k])
	}
	return strings.Join(parts, ", ")
}

func healthCheckToMap(hc HealthCheckConfig) map[string]any {
	m := map[string]any{
		"name": hc.Name,
		"type": hc.Type,
	}
	set := func(k, v string) {
		if v != "" {
			m[k] = v
		}
	}
	set("address", hc.Address)
	set("unit", hc.Unit)
	set("container", hc.Container)
	set("interface", hc.Interface)
	set("username", hc.Username)
	set("password", hc.Password)
	set("command", hc.Command)
	if hc.Interval > 0 {
		m["interval"] = hc.Interval
	}
	if hc.Timeout > 0 {
		m["timeout"] = hc.Timeout
	}
	if hc.Failures > 0 {
		m["failures"] = hc.Failures
	}
	if hc.Type == HealthHTTP && hc.ExpectStatus > 0 {
		m["expect_status"] = hc.ExpectStatus
	}
	if len(hc.Headers) > 0 {
		h := make(map[string]any, len(hc.Headers))
		for k, v := range hc.Headers {
			h[k] = v
		}
		m["headers"] = h
	}
	return m
}

func healthCheckFromMap(m map[string]any) HealthCheckConfig {
	str := func(k string) string {
		s, _ := m[k].(string)
		return s
	}
	num := func(k string) int {
		switch v := m[k].(type) {
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
		return 0
	}
	hc := HealthCheckConfig{
		Name:         str("name"),
		Type:         strings.ToLower(str("type")),
		Address:      str("address"),
		Unit:         str("unit"),
		Container:    str("container"),
		Interface:    str("interface"),
		Interval:     num("interval"),
		Timeout:      num("timeout"),
		Failures:     num("failures"),
		Password:     str("password"),
		Username:     str("username"),
		ExpectStatus: num("expect_status"),
		Command:      str("command"),
	}
	if h, ok := m["headers"].(map[string]any); ok && len(h) > 0 {
		hc.Headers = make(map[string]string, len(h))
		for k, v := range h {
			if s, ok := v.(string); ok {
				hc.Headers[k] = s
			}
		}
	}
	return hc.WithDefaults()
}
