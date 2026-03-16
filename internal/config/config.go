package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all user-configurable settings.
type Config struct {
	UnlockCombo      []string `json:"unlock_combo"`
	IdleTimeoutMin   int      `json:"idle_timeout_min"`
	MouseCorner      bool     `json:"mouse_corner"`
	EscHold          bool     `json:"esc_hold"`
	LockHotkey       []string `json:"lock_hotkey"`        // e.g. ["CTRL","ALT","L"], empty = disabled
	LockHotkeyToggle bool     `json:"lock_hotkey_toggle"` // false = lock-only, true = same hotkey toggles lock/unlock
	Notifications    bool     `json:"notifications"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		UnlockCombo:      []string{"CTRL", "ALT", "U"},
		IdleTimeoutMin:   60,
		MouseCorner:      false,
		EscHold:          true,
		LockHotkeyToggle: false,
		Notifications:    true,
	}
}

// Dir returns the config directory (%APPDATA%\kbdlock).
func Dir() (string, error) {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		return "", errors.New("APPDATA environment variable not set")
	}
	return filepath.Join(appdata, "kbdlock"), nil
}

// Path returns the full path to config.json.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads config from disk, creating defaults if the file doesn't exist.
func Load() (Config, error) {
	cfgPath, err := Path()
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(cfgPath)
	if errors.Is(err, os.ErrNotExist) {
		cfg := DefaultConfig()
		if saveErr := Save(cfg); saveErr != nil {
			return cfg, fmt.Errorf("creating default config: %w", saveErr)
		}
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("reading config: %w", err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}

	if err := validate(cfg); err != nil {
		return Config{}, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

// Save writes config to disk atomically (write temp, rename).
func Save(cfg Config) error {
	cfgPath, err := Path()
	if err != nil {
		return err
	}

	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	tmp := cfgPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("writing temp config: %w", err)
	}

	if err := os.Rename(tmp, cfgPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming config: %w", err)
	}
	return nil
}

// validate checks config invariants.
func validate(cfg Config) error {
	if len(cfg.UnlockCombo) < 2 {
		return errors.New("unlock combo must have at least 2 keys")
	}

	// Verify all key names are valid.
	if _, err := ComboToVKCodes(cfg.UnlockCombo); err != nil {
		return err
	}

	if cfg.IdleTimeoutMin < 1 {
		return errors.New("idle timeout must be at least 1 minute")
	}

	if len(cfg.LockHotkey) > 0 {
		if _, _, err := SplitHotkeyCombo(cfg.LockHotkey); err != nil {
			return fmt.Errorf("lock_hotkey: %w", err)
		}
	}

	return nil
}
