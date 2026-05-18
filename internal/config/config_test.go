package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()
	if cfg.Concurrency != 4 {
		t.Errorf("expected default concurrency 4, got %d", cfg.Concurrency)
	}
	if !cfg.Storage.Enabled {
		t.Errorf("expected storage default to be enabled")
	}
	if cfg.Paths.Python != "python3" {
		t.Errorf("expected python3, got %s", cfg.Paths.Python)
	}
	if cfg.Storage.MaxRecords != 1000 {
		t.Errorf("expected default max_records 1000, got %d", cfg.Storage.MaxRecords)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
	// Default config should still be returned even on error.
	if cfg.Concurrency != 4 {
		t.Errorf("expected default concurrency 4, got %d", cfg.Concurrency)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("{bad: yaml: :"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err == nil {
		t.Error("expected parse error for invalid YAML, got nil")
	}
	if cfg.Concurrency != 4 {
		t.Errorf("expected default concurrency 4 on parse error, got %d", cfg.Concurrency)
	}
}

func TestEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "env-config.yaml")
	yamlContent := "concurrency: 16\ntimeouts:\n  command_seconds: 60\n"
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CT_CONFIG", path)
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load via env override failed: %v", err)
	}
	if cfg.Concurrency != 16 {
		t.Errorf("expected concurrency 16 from env config, got %d", cfg.Concurrency)
	}
	if cfg.Timeouts.CommandSeconds != 60 {
		t.Errorf("expected command_seconds 60 from env config, got %d", cfg.Timeouts.CommandSeconds)
	}
}

func TestDefaultsFill(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.yaml")
	if err := os.WriteFile(path, []byte("concurrency: 8\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load partial config failed: %v", err)
	}
	if cfg.Concurrency != 8 {
		t.Errorf("expected concurrency 8 from file, got %d", cfg.Concurrency)
	}
	if cfg.Timeouts.CommandSeconds != 120 {
		t.Errorf("expected default command_seconds 120, got %d", cfg.Timeouts.CommandSeconds)
	}
	if cfg.Output.JSON != false {
		t.Errorf("expected default output.json false, got %v", cfg.Output.JSON)
	}
	if !cfg.Storage.Enabled {
		t.Error("expected default storage.enabled true")
	}
}
