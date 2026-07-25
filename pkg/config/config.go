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
	PruneUnusedModels bool    `json:"pruneUnusedModels"`
	KeepModel        []string `json:"keepModel"`

	// NativeEnums renders the listed definitions as real Dart enums instead of
	// string wrappers, keyed by definition name:
	//
	//   "nativeEnums": { "DevicePlatform": ["web", "ios", "android"] }
	//
	// The values belong in the spec, and a definition that carries its own
	// `enum` list is used in preference to anything here. This exists because
	// go-swagger does not emit enum members for a named string type, so
	// without it the members have to be maintained inside the *generated*
	// file — which gets clobbered on every regeneration and, in practice,
	// silently drifted from the backend.
	//
	// Delete an entry as soon as the spec starts carrying the values.
	NativeEnums map[string][]string `json:"nativeEnums"`
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
