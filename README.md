# kbdlock

A Windows utility that globally blocks keyboard input with multiple fail-safe unlock mechanisms. The core safety guarantee: **the keyboard can never remain permanently locked**.

## Installation

### Scoop

```sh
scoop bucket add kbdlock https://github.com/elliot40404/scoop-kbdlock
scoop install kbdlock
```

### Winget

```sh
winget install elliot40404.kbdlock
```

### Manual

Download `kbdlock.exe` from the [latest release](https://github.com/elliot40404/kbdlock/releases/latest) and place it anywhere on your PATH. That's the only file you need — the sentinel process is embedded and auto-extracted at runtime.

## Architecture

Two-process design:

- **`kbdlock-hook.exe`** (C++ sentinel) — Minimal process that installs a low-level keyboard hook. No GC, no runtime overhead. Handles combo detection, ESC hold detection, and heartbeat auto-unlock. **Embedded inside the Go binary** and auto-extracted to `%APPDATA%\kbdlock\bin\` on first run (re-extracted only when the version changes).
- **`kbdlock.exe`** (Go controller) — Manages the system tray UI, config, logging, safety monitors (mouse corner, idle timeout), and sentinel lifecycle.

They communicate over a named pipe (`\\.\pipe\kbdlock`).

## 6 Independent Unlock Paths

| # | Mechanism | Trigger |
|---|---|---|
| 1 | Configurable combo | e.g. Ctrl+Alt+U (set in config) |
| 2 | Emergency combo | Ctrl+Shift+Alt+F12 (hardcoded, always works) |
| 3 | ESC hold | Hold ESC for 5 seconds |
| 4 | Mouse corner | Move cursor to top-left (0,0) for 2 seconds |
| 5 | Tray menu | Click "Unlock Keyboard" in system tray |
| 6 | Kill process | Task Manager / `taskkill /IM kbdlock-hook.exe` |

**Fail-open design:** Lock state is memory-only. If the sentinel dies, the hook disappears and the keyboard works. If the controller dies, the sentinel auto-unlocks after 60 seconds (heartbeat timeout).

## Build

```sh
just
```

This produces a single `build/kbdlock.exe` with the sentinel binary embedded inside.

Build individually:

```sh
just sentinel    # C++ hook process
just controller  # Go controller (embeds sentinel)
```

## Usage

1. Run `kbdlock.exe` — it starts the tray app in the background and returns your terminal immediately.
2. Right-click the tray icon → **Lock Keyboard** to engage the lock.
3. Use any of the 6 unlock paths to disengage.

### CLI

```sh
kbdlock start                 # start the tray app in the background
kbdlock stop                  # stop the background tray app
kbdlock lock                  # lock via the background tray app
kbdlock unlock                # unlock via the background tray app
kbdlock help                  # show CLI help
kbdlock version              # print version
kbdlock config show           # show current config as JSON
kbdlock config set <key> <v>  # set a config value
kbdlock config reset          # reset config to defaults
```

Examples:

```sh
kbdlock.exe                   # start in background and return to the shell
kbdlock start                 # explicit script-friendly background start
kbdlock lock                  # lock if the background app is already running
kbdlock unlock                # unlock if the background app is already running
kbdlock stop                  # stop the background app
kbdlock config set mouse_corner true
kbdlock config set lock_hotkey CTRL,ALT,L
kbdlock config set lock_hotkey_toggle true
kbdlock config set notifications false
kbdlock config set lock_hotkey none        # disable hotkey
```

## Configuration

Config is stored at `%APPDATA%\kbdlock\config.json` (created with defaults on first run):

```json
{
  "unlock_combo": ["CTRL", "ALT", "U"],
  "idle_timeout_min": 60,
  "mouse_corner": false,
  "esc_hold": true,
  "lock_hotkey": [],
  "lock_hotkey_toggle": false,
  "notifications": true
}
```

- **unlock_combo** — Key combination to unlock (minimum 2 keys). Supported names: CTRL, ALT, SHIFT, A-Z, F1-F12, ESC, TAB, SPACE, ENTER, DELETE, BACKSPACE.
- **idle_timeout_min** — Auto-unlock after this many minutes (minimum 1).
- **mouse_corner** — Enable/disable the mouse-corner unlock (default: `false`).
- **esc_hold** — Enable/disable the ESC-hold unlock.
- **lock_hotkey** — Global hotkey to lock the keyboard, e.g. `["CTRL","ALT","L"]`. Empty array = disabled (default).
- **lock_hotkey_toggle** — If `true`, the `lock_hotkey` also unlocks when already locked. Default: `false`.
- **notifications** — Show Windows toast notifications on lock/unlock (default: `true`).

## License

MIT
