package plugins

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed python/*
var pluginFS embed.FS

var (
	cacheDir string
	cacheMu  sync.Mutex
)

// CacheDir returns the OS-specific cache directory used for extracted plugins.
func CacheDir() (string, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	return cacheDirLocked()
}

func cacheDirLocked() (string, error) {
	if cacheDir != "" {
		return cacheDir, nil
	}
	root, err := os.UserCacheDir()
	if err != nil || root == "" {
		return "", fmt.Errorf("cannot determine user cache directory")
	}
	cacheDir = filepath.Join(root, "atlas-recon", "plugins")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("create plugin cache directory: %w", err)
	}
	return cacheDir, nil
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
	safeName := filepath.Base(name)
	if safeName == "." || safeName == "" || safeName != name || strings.Contains(name, "..") || strings.Contains(name, "\\") {
		return "", fmt.Errorf("invalid plugin name: %s", name)
	}

	cacheMu.Lock()
	defer cacheMu.Unlock()
	dir, err := cacheDirLocked()
	if err != nil {
		return "", err
	}

	// Read the embedded file
	data, err := fs.ReadFile(pluginFS, filepath.Join("python", safeName))
	if err != nil {
		return "", fmt.Errorf("plugin not found: %s", name)
	}

	// Write to cache
	cachePath := filepath.Join(dir, safeName)

	// Check if already extracted and up-to-date
	if existingData, err := os.ReadFile(cachePath); err == nil {
		if bytes.Equal(data, existingData) {
			return cachePath, nil
		}
	}

	if err := os.WriteFile(cachePath, data, 0700); err != nil {
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
