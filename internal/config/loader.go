package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

func SaveConfig(path string, cfg Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	enc.Indent = ""
	return enc.Encode(ConfigToMap(cfg))
}

var defaultConfigPaths = []string{
	"./config/config.toml",
	"/etc/pidex/config.toml",
	"~/.config/pidex/config.toml",
}

func LoadConfig(path string) Config {
	if path != "" {
		return readConfig(path)
	}

	for _, candidate := range defaultConfigPaths {
		expanded := os.ExpandEnv(candidate)
		if expanded[0] == '~' {
			home, _ := os.UserHomeDir()
			expanded = filepath.Join(home, expanded[1:])
		}
		if _, err := os.Stat(expanded); err == nil {
			return readConfig(expanded)
		}
	}

	return DefaultConfig()
}

func readConfig(path string) Config {
	var data map[string]any
	if _, err := toml.DecodeFile(path, &data); err != nil {
		return DefaultConfig()
	}

	envToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	envChatID := os.Getenv("TELEGRAM_CHAT_ID")

	return ConfigFromMap(data, envToken, envChatID)
}
