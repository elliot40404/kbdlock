package notify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuotePowerShellEscapesSingleQuotes(t *testing.T) {
	got := quotePowerShell(`C:\Users\O'Brien\kbdlock.exe`)
	want := `'C:\Users\O''Brien\kbdlock.exe'`
	if got != want {
		t.Fatalf("quotePowerShell() = %q, want %q", got, want)
	}
}

func TestBuildShortcutScriptContainsExpectedValues(t *testing.T) {
	script := buildShortcutScript(
		`C:\Users\Test\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\kbdlock.lnk`,
		`C:\Tools\kbdlock.exe`,
		appID,
		shortcutDescription,
	)

	for _, fragment := range []string{
		"$shortcutPath = 'C:\\Users\\Test\\AppData\\Roaming\\Microsoft\\Windows\\Start Menu\\Programs\\kbdlock.lnk'",
		"$targetPath = 'C:\\Tools\\kbdlock.exe'",
		"$appID = '" + appID + "'",
		"AppUserModelIDKey",
		"[ShortcutHelper]::CreateShortcut($shortcutPath, $targetPath, $workingDirectory, $description, $appID)",
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("buildShortcutScript() missing %q in script:\n%s", fragment, script)
		}
	}
}

func TestEnsureReadyCreatesShortcut(t *testing.T) {
	tempAppData := t.TempDir()
	t.Setenv("APPDATA", tempAppData)

	prevExecutableFn := executableFn
	executableFn = os.Executable
	t.Cleanup(func() {
		executableFn = prevExecutableFn
	})

	if err := EnsureReady(); err != nil {
		t.Fatalf("EnsureReady() error = %v", err)
	}

	shortcutPath := filepath.Join(tempAppData, "Microsoft", "Windows", "Start Menu", "Programs", shortcutName)
	if _, err := os.Stat(shortcutPath); err != nil {
		t.Fatalf("shortcut not created at %s: %v", shortcutPath, err)
	}
}
