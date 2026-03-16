package controller

import (
	"testing"

	"github.com/elliot40404/kbdlock/internal/config"
	"github.com/elliot40404/kbdlock/internal/safety"
)

type fakeLog struct{}

func (fakeLog) Info(string, ...any)  {}
func (fakeLog) Error(string, ...any) {}

type fakeClient struct {
	lockCalls   int
	unlockCalls int
	locked      bool
}

func (f *fakeClient) Lock() error {
	f.lockCalls++
	f.locked = true
	return nil
}

func (f *fakeClient) Unlock() error {
	f.unlockCalls++
	f.locked = false
	return nil
}

func (f *fakeClient) Status() (bool, error) {
	return f.locked, nil
}

type fakeManager struct {
	states []bool
}

func (f *fakeManager) SetDesiredLock(locked bool) {
	f.states = append(f.states, locked)
}

func TestHandleHotkeyLocksWhenUnlocked(t *testing.T) {
	ctrl, client, mgr := newTestController(t, false, false)

	ctrl.HandleHotkey()

	if client.lockCalls != 1 {
		t.Fatalf("lockCalls = %d, want 1", client.lockCalls)
	}
	if !ctrl.locked {
		t.Fatalf("locked = false, want true")
	}
	if len(mgr.states) != 1 || !mgr.states[0] {
		t.Fatalf("desired lock states = %v, want [true]", mgr.states)
	}
}

func TestHandleHotkeyDoesNotUnlockWhenToggleDisabled(t *testing.T) {
	ctrl, client, mgr := newTestController(t, true, false)

	ctrl.HandleHotkey()

	if client.unlockCalls != 0 {
		t.Fatalf("unlockCalls = %d, want 0", client.unlockCalls)
	}
	if !ctrl.locked {
		t.Fatalf("locked = false, want true")
	}
	if len(mgr.states) != 0 {
		t.Fatalf("desired lock states = %v, want no changes", mgr.states)
	}
}

func TestHandleHotkeyUnlocksWhenToggleEnabled(t *testing.T) {
	ctrl, client, mgr := newTestController(t, true, true)

	ctrl.HandleHotkey()

	if client.unlockCalls != 1 {
		t.Fatalf("unlockCalls = %d, want 1", client.unlockCalls)
	}
	if ctrl.locked {
		t.Fatalf("locked = true, want false")
	}
	if len(mgr.states) != 1 || mgr.states[0] {
		t.Fatalf("desired lock states = %v, want [false]", mgr.states)
	}
}

func newTestController(t *testing.T, initiallyLocked, hotkeyToggle bool) (*Controller, *fakeClient, *fakeManager) {
	t.Helper()

	prevTrayState := setTrayState
	setTrayState = func(bool) {}
	t.Cleanup(func() {
		setTrayState = prevTrayState
	})

	client := &fakeClient{locked: initiallyLocked}
	mgr := &fakeManager{}

	return &Controller{
		log:         fakeLog{},
		cfg:         config.Config{IdleTimeoutMin: 1, LockHotkeyToggle: hotkeyToggle, Notifications: false},
		sentinelMgr: mgr,
		client:      client,
		idleTimeout: safety.NewIdleTimeout(1, func() {}),
		locked:      initiallyLocked,
	}, client, mgr
}
