package plugins

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

//go:embed python/*
var pluginFS embed.FS

var (
	cacheDir string
	cacheMu  sync.Mutex
)

func init() {
	home, err := os.UserHomeDir()
	if err == nil {
		cacheDir = filepath.Join(home, ".cache", "ct_plugins")
		_ = os.MkdirAll(cacheDir, 0755)
	}
}

// GetPluginPath returns the path to a plugin script, extracting it from
// the embedded filesystem if necessary.
func GetPluginPath(name string) (string, error) {
	// First, try to find it in the filesystem (development mode)
	rel := filepath.Join("plugins", "python", name)
	if _, err := os.Stat(rel); err == nil {
		return rel, nil
	}

	// Try relative to executable (production mode before embedding was available)
	exe, err := os.Executable()
	if err == nil {
		exePath := filepath.Dir(exe)
		absPath := filepath.Join(exePath, "plugins", "python", name)
		if _, err := os.Stat(absPath); err == nil {
			return absPath, nil
		}
	}

	// Extract from embedded filesystem
	return extractPlugin(name)
}

func extractPlugin(name string) (string, error) {
	if cacheDir == "" {
		return "", fmt.Errorf("cannot determine plugin cache directory")
	}

	cacheMu.Lock()
	defer cacheMu.Unlock()

	// Read the embedded file
	data, err := fs.ReadFile(pluginFS, filepath.Join("python", name))
	if err != nil {
		return "", fmt.Errorf("plugin not found: %s", name)
	}

	// Write to cache
	cachePath := filepath.Join(cacheDir, name)
	if err := os.WriteFile(cachePath, data, 0755); err != nil {
		return "", fmt.Errorf("failed to extract plugin: %w", err)
	}

	return cachePath, nil
}

// ListPlugins returns a list of all available plugins
func ListPlugins() ([]string, error) {
	var plugins []string
	err := fs.WalkDir(pluginFS, "python", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".py" {
			plugins = append(plugins, filepath.Base(path))
		}
		return nil
	})
	return plugins, err
}
