package sentinel

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elliot40404/kbdlock/assets"
	"github.com/elliot40404/kbdlock/internal/config"
	"github.com/elliot40404/kbdlock/internal/logger"
)

const (
	hashFile = "kbdlock-hook.sha256"
)

// sentinelDir returns %APPDATA%\kbdlock\bin\.
func sentinelDir() (string, error) {
	cfgDir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "bin"), nil
}

// ExtractSentinel ensures the embedded sentinel exe is extracted and up-to-date.
// Returns the path to the extracted exe.
func ExtractSentinel(log *logger.Logger) (string, error) {
	dir, err := sentinelDir()
	if err != nil {
		return "", fmt.Errorf("sentinel dir: %w", err)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create sentinel dir: %w", err)
	}

	exePath := filepath.Join(dir, sentinelExe)
	hashPath := filepath.Join(dir, hashFile)

	// Compute hash of embedded binary.
	sum := sha256.Sum256(assets.SentinelExe)
	wantHash := hex.EncodeToString(sum[:])

	// Clean up stale .old/.tmp files from previous runs.
	cleanStale(dir, log)

	// Check if current extraction is up-to-date.
	if gotHash, err := os.ReadFile(hashPath); err == nil {
		if strings.TrimSpace(string(gotHash)) == wantHash {
			// Verify the exe actually exists.
			if _, err := os.Stat(exePath); err == nil {
				log.Info("sentinel binary up-to-date, skipping extraction")
				return exePath, nil
			}
		}
	}

	// Need to extract (first run or upgrade).
	log.Info("extracting sentinel binary to %s", exePath)

	// If old exe exists (upgrade), rename it out of the way.
	if _, err := os.Stat(exePath); err == nil {
		oldPath := exePath + ".old"
		if err := os.Rename(exePath, oldPath); err != nil {
			// If rename fails, try to remove directly.
			_ = os.Remove(exePath)
		}
	}

	// Write new exe via temp file for atomicity.
	tmpPath := exePath + ".tmp"
	if err := os.WriteFile(tmpPath, assets.SentinelExe, 0o600); err != nil {
		return "", fmt.Errorf("write sentinel binary: %w", err)
	}
	if err := os.Rename(tmpPath, exePath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("rename sentinel binary: %w", err)
	}

	// Write hash sidecar.
	if err := os.WriteFile(hashPath, []byte(wantHash), 0o600); err != nil {
		return "", fmt.Errorf("write sentinel hash: %w", err)
	}

	log.Info("sentinel binary extracted successfully")
	return exePath, nil
}

// cleanStale removes leftover .old and .tmp files from interrupted runs.
func cleanStale(dir string, log *logger.Logger) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".old") || strings.HasSuffix(name, ".tmp") {
			path := filepath.Join(dir, name)
			if err := os.Remove(path); err == nil {
				log.Info("cleaned up stale file: %s", name)
			}
		}
	}
}
