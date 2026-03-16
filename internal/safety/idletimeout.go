package safety

import (
	"sync"
	"time"
)

// IdleTimeout fires an unlock callback after a configured duration.
type IdleTimeout struct {
	duration time.Duration
	onUnlock func()
	mu       sync.Mutex
	timer    *time.Timer
}

// NewIdleTimeout creates an idle timeout that calls onUnlock after the given duration.
func NewIdleTimeout(minutes int, onUnlock func()) *IdleTimeout {
	return &IdleTimeout{
		duration: time.Duration(minutes) * time.Minute,
		onUnlock: onUnlock,
	}
}

// Start begins the idle timer. Safe to call multiple times (resets the timer).
func (t *IdleTimeout) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.timer != nil {
		t.timer.Stop()
	}
	t.timer = time.AfterFunc(t.duration, t.onUnlock)
}

// Stop cancels the idle timer.
func (t *IdleTimeout) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
}
