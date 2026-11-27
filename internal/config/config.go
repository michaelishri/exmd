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

// Dir returns the directory holding the tool's on-disk state: the config
// file and saved sessions. XDG_CONFIG_HOME wins when set, on every platform;
// otherwise os.UserConfigDir supplies the platform default.
func Dir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		var err error
		if base, err = os.UserConfigDir(); err != nil {
			return "", err
		}
	}
	return filepath.Join(base, "exmd"), nil
}

// Path returns the config file location.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
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

// Save writes the config atomically, creating its parent directory when
// needed: a temporary file in the same directory is renamed into place, so
// a crash cannot leave a half-written config.
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
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
