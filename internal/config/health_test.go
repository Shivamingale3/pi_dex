package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExampleConfigHealthChecks(t *testing.T) {
	cfg := LoadConfig("../../config/config.toml.example")
	if len(cfg.HealthChecks) != 2 {
		t.Fatalf("got %d health checks, want 2", len(cfg.HealthChecks))
	}
	if err := ValidateHealthChecks(cfg.HealthChecks); err != nil {
		t.Fatal(err)
	}
	ts, redis := cfg.HealthChecks[0], cfg.HealthChecks[1]
	if ts.Name != "tailscale-ip" || ts.Type != HealthInterface || ts.Interface != "tailscale0" || ts.Failures != 6 || ts.Interval != 30 {
		t.Errorf("tailscale-ip = %+v", ts)
	}
	if redis.Address != "100.83.1.25:6379" || redis.Unit != "redis-cache.service" ||
		redis.Interval != DefaultHealthInterval || redis.Timeout != DefaultHealthTimeout || redis.Failures != DefaultHealthFailures {
		t.Errorf("redis-cache = %+v", redis)
	}
}

func TestHealthChecksRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.HealthChecks = []HealthCheckConfig{
		{Name: "api", Type: HealthHTTP, Address: "http://127.0.0.1:8080/health", Interval: 10, Timeout: 2, Failures: 2,
			ExpectStatus: 204, Headers: map[string]string{"Authorization": "Bearer secret"}},
		{Name: "cache", Type: HealthRedis, Address: "127.0.0.1:6379", Username: "mon", Password: "pw",
			Interval: 30, Timeout: 5, Failures: 3},
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0640 {
		t.Errorf("new config mode = %v, want 0640", info.Mode().Perm())
	}

	got := LoadConfig(path).HealthChecks
	if len(got) != 2 {
		t.Fatalf("got %d checks", len(got))
	}
	if got[0].Headers["Authorization"] != "Bearer secret" || got[0].ExpectStatus != 204 || got[0].Interval != 10 {
		t.Errorf("http check = %+v", got[0])
	}
	if got[1].Password != "pw" || got[1].Username != "mon" {
		t.Errorf("redis check = %+v", got[1])
	}
}

func TestValidateHealthCheck(t *testing.T) {
	base := func(typ string) HealthCheckConfig {
		return HealthCheckConfig{Name: "x", Type: typ}.WithDefaults()
	}
	cases := []struct {
		name    string
		hc      HealthCheckConfig
		others  []HealthCheckConfig
		wantErr string
	}{
		{"http ok", func() HealthCheckConfig { h := base(HealthHTTP); h.Address = "https://example.com/"; return h }(), nil, ""},
		{"http needs url", func() HealthCheckConfig { h := base(HealthHTTP); h.Address = "example.com"; return h }(), nil, "URL"},
		{"tcp needs port", func() HealthCheckConfig { h := base(HealthTCP); h.Address = "10.0.0.1"; return h }(), nil, "host:port"},
		{"redis ok", func() HealthCheckConfig { h := base(HealthRedis); h.Address = "10.0.0.1:6379"; return h }(), nil, ""},
		{"systemd needs unit", base(HealthSystemd), nil, "unit is required"},
		{"docker needs container", base(HealthDocker), nil, "container is required"},
		{"command needs command", base(HealthCommand), nil, "command is required"},
		{"interface needs interface", base(HealthInterface), nil, "interface is required"},
		{"unknown type", base("ping"), nil, "unknown type"},
		{"missing name", HealthCheckConfig{Type: HealthSystemd, Unit: "a"}.WithDefaults(), nil, "name is required"},
		{"duplicate name", func() HealthCheckConfig { h := base(HealthSystemd); h.Unit = "a"; return h }(),
			[]HealthCheckConfig{{Name: "x"}}, "already exists"},
		{"interval too small", func() HealthCheckConfig { h := base(HealthSystemd); h.Unit = "a"; h.Interval = 4; return h }(), nil, "at least 5"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.hc.Validate(c.others)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, c.wantErr)
			}
		})
	}
}

func TestMaskSecrets(t *testing.T) {
	if MaskSecret("hunter2") != "***" || MaskSecret("") != "" {
		t.Error("MaskSecret")
	}
	got := MaskedHeaders(map[string]string{"X-Token": "abc", "Authorization": "Bearer z"})
	if got != "Authorization: ***, X-Token: ***" {
		t.Errorf("MaskedHeaders = %q", got)
	}
}

func TestUnitName(t *testing.T) {
	for in, want := range map[string]string{"redis": "redis.service", "redis.service": "redis.service", "a.timer": "a.timer", "": ""} {
		if got := UnitName(in); got != want {
			t.Errorf("UnitName(%q) = %q, want %q", in, got, want)
		}
	}
}
