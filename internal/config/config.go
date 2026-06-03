package config

import "github.com/leadows/pi_dex/internal/core"

type Config struct {
	TelegramToken  string
	TelegramChatID string

	CPUInterval  int
	RAMInterval  int
	TempInterval int
	DiskInterval int

	CPUWarn     float64
	CPUCritical float64
	RAMWarn     float64
	RAMCritical float64
	DiskWarn    float64
	DiskCritical float64
	TempWarn    float64
	TempCritical float64

	ServiceWatch   []string
	ContainerWatch []string

	MonitorSSH         bool
	MonitorSudo        bool
	MonitorSystemd     bool
	MonitorDocker      bool
	MonitorNetwork     bool
	MonitorCPU         bool
	MonitorRAM         bool
	MonitorDisk        bool
	MonitorTemperature bool

	CooldownOverrides map[string]float64
}

func DefaultConfig() Config {
	return Config{
		CPUInterval:  core.DefaultCPUInterval,
		RAMInterval:  core.DefaultRAMInterval,
		TempInterval: core.DefaultTempInterval,
		DiskInterval: core.DefaultDiskInterval,

		CPUWarn:     core.DefaultCPUWarn,
		CPUCritical: core.DefaultCPUCritical,
		RAMWarn:     core.DefaultRAMWarn,
		RAMCritical: core.DefaultRAMCritical,
		DiskWarn:    core.DefaultDiskWarn,
		DiskCritical: core.DefaultDiskCritical,
		TempWarn:    core.DefaultTempWarn,
		TempCritical: core.DefaultTempCritical,

		MonitorSSH:         true,
		MonitorSudo:        true,
		MonitorSystemd:     true,
		MonitorDocker:      true,
		MonitorNetwork:     true,
		MonitorCPU:         true,
		MonitorRAM:         true,
		MonitorDisk:        true,
		MonitorTemperature: true,
	}
}

func ConfigFromMap(data map[string]any, envToken, envChatID string) Config {
	cfg := DefaultConfig()

	if token, ok := data["telegram"].(map[string]any); ok {
		if t, ok := token["bot_token"].(string); ok && t != "" && envToken == "" {
			cfg.TelegramToken = t
		}
		if c, ok := token["chat_id"].(string); ok && c != "" && envChatID == "" {
			cfg.TelegramChatID = c
		}
	}
	if envToken != "" {
		cfg.TelegramToken = envToken
	}
	if envChatID != "" {
		cfg.TelegramChatID = envChatID
	}

	if m, ok := data["monitor"].(map[string]any); ok {
		if v, ok := m["ssh"].(bool); ok {
			cfg.MonitorSSH = v
		}
		if v, ok := m["sudo"].(bool); ok {
			cfg.MonitorSudo = v
		}
		if v, ok := m["systemd"].(bool); ok {
			cfg.MonitorSystemd = v
		}
		if v, ok := m["docker"].(bool); ok {
			cfg.MonitorDocker = v
		}
		if v, ok := m["network"].(bool); ok {
			cfg.MonitorNetwork = v
		}
		if v, ok := m["cpu"].(bool); ok {
			cfg.MonitorCPU = v
		}
		if v, ok := m["ram"].(bool); ok {
			cfg.MonitorRAM = v
		}
		if v, ok := m["disk"].(bool); ok {
			cfg.MonitorDisk = v
		}
		if v, ok := m["temperature"].(bool); ok {
			cfg.MonitorTemperature = v
		}
	}

	if p, ok := data["pollers"].(map[string]any); ok {
		if v, ok := p["cpu_interval"].(int64); ok {
			cfg.CPUInterval = int(v)
		}
		if v, ok := p["ram_interval"].(int64); ok {
			cfg.RAMInterval = int(v)
		}
		if v, ok := p["temp_interval"].(int64); ok {
			cfg.TempInterval = int(v)
		}
		if v, ok := p["disk_interval"].(int64); ok {
			cfg.DiskInterval = int(v)
		}
	}

	if t, ok := data["thresholds"].(map[string]any); ok {
		if v, ok := t["cpu_warn"].(float64); ok {
			cfg.CPUWarn = v
		}
		if v, ok := t["cpu_critical"].(float64); ok {
			cfg.CPUCritical = v
		}
		if v, ok := t["ram_warn"].(float64); ok {
			cfg.RAMWarn = v
		}
		if v, ok := t["ram_critical"].(float64); ok {
			cfg.RAMCritical = v
		}
		if v, ok := t["disk_warn"].(float64); ok {
			cfg.DiskWarn = v
		}
		if v, ok := t["disk_critical"].(float64); ok {
			cfg.DiskCritical = v
		}
		if v, ok := t["temp_warn"].(float64); ok {
			cfg.TempWarn = v
		}
		if v, ok := t["temp_critical"].(float64); ok {
			cfg.TempCritical = v
		}
	}

	if s, ok := data["services"].(map[string]any); ok {
		if w, ok := s["watch"].([]any); ok {
			for _, v := range w {
				if s, ok := v.(string); ok {
					cfg.ServiceWatch = append(cfg.ServiceWatch, s)
				}
			}
		}
	}

	if c, ok := data["containers"].(map[string]any); ok {
		if w, ok := c["watch"].([]any); ok {
			for _, v := range w {
				if s, ok := v.(string); ok {
					cfg.ContainerWatch = append(cfg.ContainerWatch, s)
				}
			}
		}
	}

	if cd, ok := data["cooldowns"].(map[string]any); ok {
		cfg.CooldownOverrides = make(map[string]float64, len(cd))
		for k, v := range cd {
			switch val := v.(type) {
			case float64:
				cfg.CooldownOverrides[k] = val
			case int64:
				cfg.CooldownOverrides[k] = float64(val)
			}
		}
	}

	return cfg
}
