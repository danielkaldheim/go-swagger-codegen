package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	PubName          string   `json:"pubName"`
	PubVersion       string   `json:"pubVersion"`
	PubDescription   string   `json:"pubDescription"`
	BrowserClient    bool     `json:"browserClient"`
	UseEnumExtension bool     `json:"useEnumExtension"`
	ExcludeApi       []string `json:"excludeApi"`
	ExcludeModel     []string `json:"excludeModel"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		PubName:          "swagger",
		PubVersion:       "1.0.0",
		PubDescription:   "Swagger API client",
		BrowserClient:    false,
		UseEnumExtension: true,
	}

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
