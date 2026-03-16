package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigIncludesLockHotkeyToggle(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.LockHotkeyToggle {
		t.Fatalf("DefaultConfig().LockHotkeyToggle = true, want false")
	}
	if !cfg.Notifications {
		t.Fatalf("DefaultConfig().Notifications = false, want true")
	}
}

func TestLoadMergesDefaultsForMissingFields(t *testing.T) {
	appdata := t.TempDir()
	t.Setenv("APPDATA", appdata)

	cfgDir := filepath.Join(appdata, "kbdlock")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	data := []byte(`{
  "unlock_combo": ["CTRL", "ALT", "U"],
  "idle_timeout_min": 15,
  "mouse_corner": true,
  "esc_hold": false,
  "lock_hotkey": ["CTRL", "ALT", "L"]
}`)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.LockHotkeyToggle {
		t.Fatalf("Load().LockHotkeyToggle = true, want false for missing field")
	}
	if !cfg.Notifications {
		t.Fatalf("Load().Notifications = false, want default true for missing field")
	}
}

func TestSaveAndLoadPreservesLockHotkeyToggle(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	cfg := DefaultConfig()
	cfg.LockHotkey = []string{"CTRL", "ALT", "L"}
	cfg.LockHotkeyToggle = true

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !got.LockHotkeyToggle {
		t.Fatalf("Load().LockHotkeyToggle = false, want true")
	}
}
