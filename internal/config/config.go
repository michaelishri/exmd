// Package config stores the tool's user preferences. It deliberately holds
// only plain values: interpretation (for example, turning a theme name into
// a palette) stays with the command layer.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the on-disk preference set.
type Config struct {
	Theme string `yaml:"theme,omitempty"`

	// SaveSessions turns automatic session snapshots off when false. Unset
	// means on.
	SaveSessions *bool `yaml:"save_sessions,omitempty"`
}

// SessionsEnabled reports whether the viewer should save sessions.
func (c Config) SessionsEnabled() bool {
	return c.SaveSessions == nil || *c.SaveSessions
}

// Path returns the config file location. XDG_CONFIG_HOME wins when set;
// otherwise os.UserConfigDir supplies the platform default.
func Path() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "exmd", "config.yaml"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "exmd", "config.yaml"), nil
}

// Load reads the config. A missing file is not an error: defaults apply.
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return c, nil
}

// Save writes the config, creating its parent directory when needed.
func Save(c Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
