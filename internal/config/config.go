package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Concurrency int `yaml:"concurrency"`
	Timeouts    struct {
		CommandSeconds int `yaml:"command_seconds"`
	} `yaml:"timeouts"`
	Output struct {
		JSON bool `yaml:"json"`
	} `yaml:"output"`
	Storage struct {
		Enabled   bool   `yaml:"enabled"`
		ResultsDB string `yaml:"results_db"`
	} `yaml:"storage"`
	Paths struct {
		Python   string `yaml:"python"`
		Nmap     string `yaml:"nmap"`
		Nslookup string `yaml:"nslookup"`
		Whois    string `yaml:"whois"`
	} `yaml:"paths"`
	APIKeys map[string]string `yaml:"apikeys"`
}

func Default() Config {
	cfg := Config{Concurrency: 4}
	cfg.Timeouts.CommandSeconds = 120
	cfg.Output.JSON = false
	cfg.Storage.Enabled = true
	cfg.Storage.ResultsDB = defaultResultsPath()
	cfg.Paths.Python = "python3"
	cfg.Paths.Nmap = "nmap"
	cfg.Paths.Nslookup = "nslookup"
	cfg.Paths.Whois = "whois"
	cfg.APIKeys = make(map[string]string)
	return cfg
}

func defaultResultsPath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		return "results.db"
	}
	return filepath.Join(cacheDir, "atlas-recon", "results.db")
}

func Load(path string) (Config, error) {
	cfg := Default()
	resolved, err := resolvePath(path)
	if err != nil {
		return cfg, err
	}
	if resolved == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return cfg, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

func resolvePath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	if env := os.Getenv("CT_CONFIG"); env != "" {
		return env, nil
	}

	local := filepath.Join("configs", "default.yaml")
	if _, err := os.Stat(local); err == nil {
		return local, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return "", nil
}

// Signed-off-by: ronikoz
