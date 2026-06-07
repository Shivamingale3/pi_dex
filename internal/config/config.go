package config

import "github.com/Shivamingale3/pi_dex/internal/core"

type CustomEventConfig struct {
	Name     string
	Pattern  string
	Severity string
	Title    string
	Message  string
}

type CustomServiceConfig struct {
	Name        string
	Description string
	Events      []CustomEventConfig
}

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

	CustomServices []CustomServiceConfig
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

func ConfigToMap(cfg Config) map[string]any {
	m := map[string]any{}

	m["monitor"] = map[string]any{
		"ssh":         cfg.MonitorSSH,
		"sudo":        cfg.MonitorSudo,
		"systemd":     cfg.MonitorSystemd,
		"docker":      cfg.MonitorDocker,
		"network":     cfg.MonitorNetwork,
		"cpu":         cfg.MonitorCPU,
		"ram":         cfg.MonitorRAM,
		"disk":        cfg.MonitorDisk,
		"temperature": cfg.MonitorTemperature,
	}

	m["pollers"] = map[string]any{
		"cpu_interval":  cfg.CPUInterval,
		"ram_interval":  cfg.RAMInterval,
		"temp_interval": cfg.TempInterval,
		"disk_interval": cfg.DiskInterval,
	}

	m["thresholds"] = map[string]any{
		"cpu_warn":      cfg.CPUWarn,
		"cpu_critical":  cfg.CPUCritical,
		"ram_warn":      cfg.RAMWarn,
		"ram_critical":  cfg.RAMCritical,
		"disk_warn":     cfg.DiskWarn,
		"disk_critical": cfg.DiskCritical,
		"temp_warn":     cfg.TempWarn,
		"temp_critical": cfg.TempCritical,
	}

	if len(cfg.ServiceWatch) > 0 {
		watch := make([]any, len(cfg.ServiceWatch))
		for i, v := range cfg.ServiceWatch {
			watch[i] = v
		}
		m["services"] = map[string]any{"watch": watch}
	}

	if len(cfg.ContainerWatch) > 0 {
		watch := make([]any, len(cfg.ContainerWatch))
		for i, v := range cfg.ContainerWatch {
			watch[i] = v
		}
		m["containers"] = map[string]any{"watch": watch}
	}

	if len(cfg.CooldownOverrides) > 0 {
		cd := make(map[string]any, len(cfg.CooldownOverrides))
		for k, v := range cfg.CooldownOverrides {
			cd[k] = v
		}
		m["cooldowns"] = cd
	}

	if len(cfg.CustomServices) > 0 {
		svcs := make([]any, len(cfg.CustomServices))
		for i, svc := range cfg.CustomServices {
			sm := map[string]any{
				"name":        svc.Name,
				"description": svc.Description,
			}
			evts := make([]any, len(svc.Events))
			for j, ev := range svc.Events {
				evts[j] = map[string]any{
					"name":     ev.Name,
					"pattern":  ev.Pattern,
					"severity": ev.Severity,
					"title":    ev.Title,
					"message":  ev.Message,
				}
			}
			sm["events"] = evts
			svcs[i] = sm
		}
		m["custom_services"] = svcs
	}

	return m
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

	if cs, ok := data["custom_services"].([]any); ok {
		for _, svcRaw := range cs {
			svcMap, ok := svcRaw.(map[string]any)
			if !ok {
				continue
			}
			svc := CustomServiceConfig{}
			if n, ok := svcMap["name"].(string); ok {
				svc.Name = n
			}
			if d, ok := svcMap["description"].(string); ok {
				svc.Description = d
			}
			if evts, ok := svcMap["events"].([]any); ok {
				for _, evtRaw := range evts {
					evtMap, ok := evtRaw.(map[string]any)
					if !ok {
						continue
					}
					evt := CustomEventConfig{}
					if n, ok := evtMap["name"].(string); ok {
						evt.Name = n
					}
					if p, ok := evtMap["pattern"].(string); ok {
						evt.Pattern = p
					}
					if s, ok := evtMap["severity"].(string); ok {
						evt.Severity = s
					}
					if t, ok := evtMap["title"].(string); ok {
						evt.Title = t
					}
					if m, ok := evtMap["message"].(string); ok {
						evt.Message = m
					}
					svc.Events = append(svc.Events, evt)
				}
			}
			if svc.Name != "" && len(svc.Events) > 0 {
				cfg.CustomServices = append(cfg.CustomServices, svc)
			}
		}
	}

	return cfg
}
