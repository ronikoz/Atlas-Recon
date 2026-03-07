package config

import "testing"

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
}
