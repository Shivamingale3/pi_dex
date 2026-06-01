package config

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Telegram struct {
		Enabled bool `toml:"enabled"`
	} `toml:"telegram"`

	Cooldowns map[string]int `toml:"cooldowns"`
}

func Load(path string) (*Config, error) {

	data, err := os.ReadFile(path)

	if err != nil {
		return nil, err
	}

	var cfg Config

	err = toml.Unmarshal(data, &cfg)

	if err != nil {
		return nil, err
	}

	return &cfg, nil
}