package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/elliot40404/kbdlock/internal/config"
)

var (
	kernel32      = syscall.NewLazyDLL("kernel32.dll")
	attachConsole = kernel32.NewProc("AttachConsole")
	getStdHandle  = kernel32.NewProc("GetStdHandle")
	getFileType   = kernel32.NewProc("GetFileType")
	getLastError  = kernel32.NewProc("GetLastError")
)

const attachParentProcess = ^uint32(0) // ATTACH_PARENT_PROCESS = (DWORD)-1

const (
	// ContinueWithGUI signals that no CLI subcommand handled the invocation.
	ContinueWithGUI = -1
	// StartDetached signals that the caller should launch the tray in the background.
	StartDetached = -2
	// StopBackground signals that the caller should stop the background tray app.
	StopBackground = -3
	// LockKeyboard signals that the caller should lock the keyboard via the background app.
	LockKeyboard = -4
	// UnlockKeyboard signals that the caller should unlock the keyboard via the background app.
	UnlockKeyboard = -5
)

const (
	stdInputHandle  = ^uint32(9)  // STD_INPUT_HANDLE  = ((DWORD)-10)
	stdOutputHandle = ^uint32(10) // STD_OUTPUT_HANDLE = ((DWORD)-11)
	stdErrorHandle  = ^uint32(11) // STD_ERROR_HANDLE  = ((DWORD)-12)
	fileTypeUnknown = 0
	invalidHandle   = ^uintptr(0)
)

// Run handles CLI subcommands. Returns exit code.
// Should be called before any GUI setup.
func Run(args []string, version string) int {
	attachParentConsole()

	if len(args) == 0 {
		return ContinueWithGUI
	}

	if isHelpArg(args[0]) || args[0] == "help" {
		printUsage()
		return 0
	}

	switch args[0] {
	case "version":
		fmt.Println("kbdlock", version)
		return 0
	case "start":
		return StartDetached
	case "stop":
		return StopBackground
	case "lock":
		return LockKeyboard
	case "unlock":
		return UnlockKeyboard
	case "config":
		return runConfig(args[1:])
	default:
		fmt.Printf("unknown command: %s\n", args[0])
		printUsage()
		return 1
	}
}

func runConfig(args []string) int {
	if len(args) == 0 || args[0] == "help" || isHelpArg(args[0]) {
		printConfigUsage()
		return 0
	}

	switch args[0] {
	case "show":
		return configShow()
	case "set":
		if len(args) > 1 && isHelpArg(args[1]) {
			printConfigSetUsage()
			return 0
		}
		if len(args) < 3 {
			printConfigSetUsage()
			return 1
		}
		return configSet(args[1], args[2])
	case "reset":
		return configReset()
	default:
		fmt.Printf("unknown config command: %s\n", args[0])
		printConfigUsage()
		return 1
	}
}

func configShow() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return 1
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	fmt.Println(string(data))
	return 0
}

func configSet(key, value string) int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("error loading config: %v\n", err)
		return 1
	}

	switch key {
	case "unlock_combo":
		cfg.UnlockCombo = strings.Split(strings.ToUpper(value), ",")
	case "idle_timeout_min":
		n, err := strconv.Atoi(value)
		if err != nil {
			fmt.Printf("invalid integer: %s\n", value)
			return 1
		}
		cfg.IdleTimeoutMin = n
	case "mouse_corner":
		b, err := strconv.ParseBool(value)
		if err != nil {
			fmt.Printf("invalid boolean: %s\n", value)
			return 1
		}
		cfg.MouseCorner = b
	case "esc_hold":
		b, err := strconv.ParseBool(value)
		if err != nil {
			fmt.Printf("invalid boolean: %s\n", value)
			return 1
		}
		cfg.EscHold = b
	case "lock_hotkey":
		if strings.EqualFold(value, "none") || value == "" {
			cfg.LockHotkey = nil
		} else {
			cfg.LockHotkey = strings.Split(strings.ToUpper(value), ",")
		}
	case "lock_hotkey_toggle":
		b, err := strconv.ParseBool(value)
		if err != nil {
			fmt.Printf("invalid boolean: %s\n", value)
			return 1
		}
		cfg.LockHotkeyToggle = b
	case "notifications":
		b, err := strconv.ParseBool(value)
		if err != nil {
			fmt.Printf("invalid boolean: %s\n", value)
			return 1
		}
		cfg.Notifications = b
	default:
		fmt.Printf("unknown config key: %s\n", key)
		fmt.Printf("valid keys: %s\n", strings.Join(validConfigKeys(), ", "))
		return 1
	}

	if err := config.Save(cfg); err != nil {
		fmt.Printf("error saving config: %v\n", err)
		return 1
	}
	fmt.Printf("%s set to %s\n", key, value)
	return 0
}

func configReset() int {
	cfg := config.DefaultConfig()
	if err := config.Save(cfg); err != nil {
		fmt.Printf("error saving config: %v\n", err)
		return 1
	}
	fmt.Println("config reset to defaults")
	return 0
}

func printUsage() {
	fmt.Println(`usage: kbdlock <command>

commands:
  help                 show this help
  -h, --help, /?       show this help
  start                start the tray app in the background
  stop                 stop the background tray app
  lock                 lock the keyboard via the background tray app
  unlock               unlock the keyboard via the background tray app
  version              print version
  config help          show config help
  config show          show current config
  config set <k> <v>   set a config value
  config reset         reset config to defaults`)
}

func printConfigUsage() {
	fmt.Println(`usage: kbdlock config <command>

commands:
  help                 show config help
  show                 show current config as JSON
  set <key> <value>    set a config value
  reset                reset config to defaults

valid keys:
  unlock_combo         comma-separated keys, e.g. CTRL,ALT,U
  idle_timeout_min     integer minutes, minimum 1
  mouse_corner         boolean (` + "`true`/`false`" + `)
  esc_hold             boolean (` + "`true`/`false`" + `)
  lock_hotkey          comma-separated hotkey, or ` + "`none`" + ` to disable
  lock_hotkey_toggle   boolean; false = lock-only, true = toggle lock/unlock
  notifications        boolean (` + "`true`/`false`" + `)`)
}

func printConfigSetUsage() {
	fmt.Println("usage: kbdlock config set <key> <value>")
	fmt.Printf("valid keys: %s\n", strings.Join(validConfigKeys(), ", "))
}

func validConfigKeys() []string {
	return []string{
		"unlock_combo",
		"idle_timeout_min",
		"mouse_corner",
		"esc_hold",
		"lock_hotkey",
		"lock_hotkey_toggle",
		"notifications",
	}
}

func isHelpArg(arg string) bool {
	switch arg {
	case "-h", "--help", "/?":
		return true
	default:
		return false
	}
}

func attachParentConsole() {
	ret, _, _ := attachConsole.Call(uintptr(attachParentProcess))
	if ret == 0 {
		lastErr, _, _ := getLastError.Call()
		// ERROR_ACCESS_DENIED means the process already has a console.
		// ERROR_INVALID_HANDLE means there is no parent console to attach to.
		if lastErr != 5 && lastErr != 6 {
			return
		}
	}

	rebindStdHandles()
}

func rebindStdHandles() {
	if !hasValidStdHandle(stdInputHandle) {
		if in, err := os.OpenFile("CONIN$", os.O_RDONLY, 0); err == nil {
			os.Stdin = in
		}
	}
	if !hasValidStdHandle(stdOutputHandle) {
		if out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
			os.Stdout = out
		}
	}
	if !hasValidStdHandle(stdErrorHandle) {
		if errOut, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
			os.Stderr = errOut
		}
	}
}

func hasValidStdHandle(kind uint32) bool {
	handle, _, _ := getStdHandle.Call(uintptr(kind))
	if handle == 0 || handle == invalidHandle {
		return false
	}

	fileType, _, _ := getFileType.Call(handle)
	if fileType != fileTypeUnknown {
		return true
	}

	lastErr, _, _ := getLastError.Call()
	return lastErr == 0
}
