package controller

import (
	"context"
	"sync"
	"time"

	"github.com/elliot40404/kbdlock/internal/config"
	"github.com/elliot40404/kbdlock/internal/hotkey"
	"github.com/elliot40404/kbdlock/internal/ipc"
	"github.com/elliot40404/kbdlock/internal/logger"
	"github.com/elliot40404/kbdlock/internal/notify"
	"github.com/elliot40404/kbdlock/internal/safety"
	"github.com/elliot40404/kbdlock/internal/sentinel"
	"github.com/elliot40404/kbdlock/internal/tray"
)

type logSink interface {
	Info(format string, args ...any)
	Error(format string, args ...any)
}

type lockClient interface {
	Lock() error
	Unlock() error
	Status() (bool, error)
}

type desiredLockSetter interface {
	SetDesiredLock(bool)
}

var setTrayState = tray.SetState

// Controller orchestrates lock state, IPC, and safety monitors.
type Controller struct {
	log             logSink
	cfg             config.Config
	sentinelMgr     desiredLockSetter
	client          lockClient
	idleTimeout     *safety.IdleTimeout
	cornerCancel    context.CancelFunc
	hotkeyCancel    context.CancelFunc
	stateSyncCancel context.CancelFunc

	mu     sync.Mutex
	locked bool
}

// New creates a controller.
func New(log *logger.Logger, cfg config.Config, mgr *sentinel.Manager, client *ipc.Client) *Controller {
	c := &Controller{
		log:         log,
		cfg:         cfg,
		sentinelMgr: mgr,
		client:      client,
	}

	c.idleTimeout = safety.NewIdleTimeout(cfg.IdleTimeoutMin, c.Unlock)
	return c
}

// SetClient replaces the IPC client (used after sentinel restart).
func (c *Controller) SetClient(client *ipc.Client) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.client = client
}

// Lock engages the keyboard lock.
func (c *Controller) Lock() {
	_ = c.LockCommand()
}

// LockCommand engages the keyboard lock and reports failures to the caller.
func (c *Controller) LockCommand() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.locked {
		return nil
	}

	if err := c.client.Lock(); err != nil {
		c.log.Error("lock failed: %v", err)
		return err
	}

	c.locked = true
	if c.sentinelMgr != nil {
		c.sentinelMgr.SetDesiredLock(true)
	}
	setTrayState(true)
	c.idleTimeout.Start()
	c.log.Info("keyboard locked")
	c.notifyStateChange("Keyboard locked")
	return nil
}

// Unlock disengages the keyboard lock.
func (c *Controller) Unlock() {
	_ = c.UnlockCommand()
}

// UnlockCommand disengages the keyboard lock and reports failures to the caller.
func (c *Controller) UnlockCommand() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.locked {
		return nil
	}

	if err := c.client.Unlock(); err != nil {
		c.log.Error("unlock failed: %v", err)
		return err
	}

	c.locked = false
	if c.sentinelMgr != nil {
		c.sentinelMgr.SetDesiredLock(false)
	}
	setTrayState(false)
	c.idleTimeout.Stop()
	c.log.Info("keyboard unlocked")
	c.notifyStateChange("Keyboard unlocked")
	return nil
}

// syncUnlock updates controller state to unlocked without sending UNLOCK to the
// sentinel (it already unlocked itself via the hotkey combo).
func (c *Controller) syncUnlock() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.locked {
		return
	}

	c.locked = false
	if c.sentinelMgr != nil {
		c.sentinelMgr.SetDesiredLock(false)
	}
	setTrayState(false)
	c.idleTimeout.Stop()
	c.log.Info("keyboard unlocked (sentinel-side)")
	c.notifyStateChange("Keyboard unlocked")
}

// StartStateSync polls the sentinel to detect when it unlocks independently
// (e.g. via the Ctrl+Alt+U combo handled in C++).
func (c *Controller) StartStateSync(ctx context.Context) {
	syncCtx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	c.stateSyncCancel = cancel
	c.mu.Unlock()

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-syncCtx.Done():
				return
			case <-ticker.C:
				c.mu.Lock()
				isLocked := c.locked
				client := c.client
				c.mu.Unlock()

				if !isLocked {
					continue
				}

				sentinelLocked, err := client.Status()
				if err != nil {
					continue
				}

				if !sentinelLocked {
					c.syncUnlock()
				}
			}
		}
	}()
}

// StartMouseCorner begins the mouse corner safety monitor.
func (c *Controller) StartMouseCorner(ctx context.Context) {
	if !c.cfg.MouseCorner {
		return
	}

	cornerCtx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	c.cornerCancel = cancel
	c.mu.Unlock()

	monitor := safety.NewMouseCornerMonitor(c.Unlock)
	go monitor.Run(cornerCtx)
}

// HandleHotkey applies the configured global hotkey action.
func (c *Controller) HandleHotkey() {
	c.mu.Lock()
	isLocked := c.locked
	c.mu.Unlock()

	if isLocked {
		if c.cfg.LockHotkeyToggle {
			c.Unlock()
		}
		return
	}

	c.Lock()
}

// StartHotkey begins listening for the global lock/unlock hotkey.
func (c *Controller) StartHotkey(ctx context.Context) {
	if len(c.cfg.LockHotkey) == 0 {
		return
	}

	modifiers, vk, err := config.SplitHotkeyCombo(c.cfg.LockHotkey)
	if err != nil {
		c.log.Error("hotkey config: %v", err)
		return
	}

	hkCtx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	c.hotkeyCancel = cancel
	c.mu.Unlock()

	listener := hotkey.New(c.log, modifiers, vk, c.HandleHotkey)
	go listener.Run(hkCtx)
}

// Stop cleans up all monitors.
func (c *Controller) Stop() {
	c.mu.Lock()
	cornerCancel := c.cornerCancel
	hkCancel := c.hotkeyCancel
	syncCancel := c.stateSyncCancel
	c.mu.Unlock()

	if cornerCancel != nil {
		cornerCancel()
	}
	if hkCancel != nil {
		hkCancel()
	}
	if syncCancel != nil {
		syncCancel()
	}
	c.idleTimeout.Stop()
}

func (c *Controller) notifyStateChange(message string) {
	if !c.cfg.Notifications {
		return
	}

	go func() {
		if err := notify.Notify("kbdlock", message); err != nil {
			c.log.Error("notification failed: %v", err)
		}
	}()
}
