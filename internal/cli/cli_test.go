package cli

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	output := captureStdout(t, func() {
		code := Run([]string{"--help"}, "test")
		if code != 0 {
			t.Fatalf("Run() exit code = %d, want 0", code)
		}
	})

	if !strings.Contains(output, "usage: kbdlock <command>") {
		t.Fatalf("help output missing root usage: %q", output)
	}
	if !strings.Contains(output, "start                start the tray app in the background") {
		t.Fatalf("help output missing start entry: %q", output)
	}
	for _, command := range []string{
		"stop                 stop the background tray app",
		"lock                 lock the keyboard via the background tray app",
		"unlock               unlock the keyboard via the background tray app",
	} {
		if !strings.Contains(output, command) {
			t.Fatalf("help output missing %q: %q", command, output)
		}
	}
	if !strings.Contains(output, "config help") {
		t.Fatalf("help output missing config help entry: %q", output)
	}
}

func TestRunConfigHelp(t *testing.T) {
	output := captureStdout(t, func() {
		code := Run([]string{"config", "help"}, "test")
		if code != 0 {
			t.Fatalf("Run() exit code = %d, want 0", code)
		}
	})

	if !strings.Contains(output, "usage: kbdlock config <command>") {
		t.Fatalf("config help output missing usage: %q", output)
	}
	if !strings.Contains(output, "lock_hotkey_toggle") {
		t.Fatalf("config help output missing lock_hotkey_toggle: %q", output)
	}
}

func TestRunConfigSetHelp(t *testing.T) {
	output := captureStdout(t, func() {
		code := Run([]string{"config", "set", "--help"}, "test")
		if code != 0 {
			t.Fatalf("Run() exit code = %d, want 0", code)
		}
	})

	if !strings.Contains(output, "usage: kbdlock config set <key> <value>") {
		t.Fatalf("config set help output missing usage: %q", output)
	}
	if !strings.Contains(output, "lock_hotkey_toggle") {
		t.Fatalf("config set help output missing lock_hotkey_toggle: %q", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close write pipe: %v", err)
	}

	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	return string(output)
}
