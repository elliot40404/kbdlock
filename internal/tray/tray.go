package tray

import (
	"fyne.io/systray"
	"github.com/elliot40404/kbdlock/assets"
)

// Actions holds callbacks for tray menu actions.
type Actions struct {
	OnLock   func()
	OnUnlock func()
	OnReady  func()
	OnQuit   func()
}

// Tray manages the system tray icon and menu.
type Tray struct {
	actions    Actions
	lockItem   *systray.MenuItem
	unlockItem *systray.MenuItem
}

// Run starts the system tray. This blocks until the tray exits.
// Call from main goroutine.
func Run(actions Actions) {
	t := &Tray{actions: actions}
	systray.Run(t.onReady, t.onExit)
}

// Quit requests the tray event loop to exit.
func Quit() {
	systray.Quit()
}

// Version is set by main to display in the tray tooltip.
var Version = "dev"

// global reference so SetState can update menu items.
var activeTray *Tray

// SetState updates the tray icon and menu to reflect lock state.
func SetState(locked bool) {
	if locked {
		systray.SetIcon(assets.LockedIcon)
		systray.SetTooltip("kbdlock " + Version + " — Keyboard LOCKED")
	} else {
		systray.SetIcon(assets.UnlockedIcon)
		systray.SetTooltip("kbdlock " + Version + " — Keyboard unlocked")
	}
	if t := activeTray; t != nil {
		if locked {
			t.lockItem.Disable()
			t.unlockItem.Enable()
		} else {
			t.lockItem.Enable()
			t.unlockItem.Disable()
		}
	}
}

func (t *Tray) onReady() {
	systray.SetIcon(assets.UnlockedIcon)
	systray.SetTitle("kbdlock")
	systray.SetTooltip("kbdlock " + Version + " — Keyboard unlocked")

	t.lockItem = systray.AddMenuItem("Lock Keyboard", "Lock all keyboard input")
	t.unlockItem = systray.AddMenuItem("Unlock Keyboard", "Unlock keyboard input")
	t.unlockItem.Disable() // starts unlocked, so "Unlock" is disabled
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Quit", "Exit kbdlock")

	activeTray = t
	if t.actions.OnReady != nil {
		t.actions.OnReady()
	}
	go t.handleClicks(quitItem)
}

func (t *Tray) handleClicks(quitItem *systray.MenuItem) {
	for {
		select {
		case <-t.lockItem.ClickedCh:
			if t.actions.OnLock != nil {
				t.actions.OnLock()
			}
		case <-t.unlockItem.ClickedCh:
			if t.actions.OnUnlock != nil {
				t.actions.OnUnlock()
			}
		case <-quitItem.ClickedCh:
			if t.actions.OnQuit != nil {
				t.actions.OnQuit()
			}
			systray.Quit()
			return
		}
	}
}

func (t *Tray) onExit() {
	// Cleanup if needed.
}
