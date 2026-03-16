package launch

import "testing"

func TestParseInternalStartupStatusPath(t *testing.T) {
	path, ok := ParseInternalStartupStatusPath([]string{InternalRunTrayArg, startupStatusFileArg, `C:\Temp\kbdlock-startup.status`})
	if !ok {
		t.Fatalf("ParseInternalStartupStatusPath() ok = false, want true")
	}
	if path != `C:\Temp\kbdlock-startup.status` {
		t.Fatalf("ParseInternalStartupStatusPath() path = %q, want startup status path", path)
	}
}

func TestParseInternalStartupStatusPathWithoutPath(t *testing.T) {
	path, ok := ParseInternalStartupStatusPath([]string{InternalRunTrayArg})
	if !ok {
		t.Fatalf("ParseInternalStartupStatusPath() ok = false, want true")
	}
	if path != "" {
		t.Fatalf("ParseInternalStartupStatusPath() path = %q, want empty", path)
	}
}

func TestIsStartCommand(t *testing.T) {
	if !IsStartCommand([]string{StartCommand}) {
		t.Fatal("IsStartCommand() = false, want true")
	}
	if IsStartCommand([]string{"help"}) {
		t.Fatal("IsStartCommand() = true, want false")
	}
}
