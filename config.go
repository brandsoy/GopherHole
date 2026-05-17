package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func loadConfig(path string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, fmt.Errorf("config not found at %s", path)
		}
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "repos.config.json"
	}
	return filepath.Join(home, ".config", "gopherhole", "repos.config.json")
}

func runInit(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists at %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	template := "{\n  \"folders\": [\n    \"/path/to/work\",\n    \"/path/to/personal\"\n  ]\n}\n"
	return os.WriteFile(path, []byte(template), 0o644)
}
