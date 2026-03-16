package safety

import (
	"context"
	"syscall"
	"time"
	"unsafe"
)

const (
	pollInterval    = 100 * time.Millisecond
	cornerThreshold = 20 // consecutive polls at (0,0) before unlock
)

var (
	user32       = syscall.NewLazyDLL("user32.dll")
	getCursorPos = user32.NewProc("GetCursorPos")
)

type point struct {
	X, Y int32
}

// MouseCornerMonitor triggers a callback when the cursor stays at (0,0).
type MouseCornerMonitor struct {
	onUnlock func()
}

// NewMouseCornerMonitor creates a monitor that calls onUnlock when the cursor
// is held at (0,0) for ~2 seconds.
func NewMouseCornerMonitor(onUnlock func()) *MouseCornerMonitor {
	return &MouseCornerMonitor{onUnlock: onUnlock}
}

// Run polls the cursor position until ctx is cancelled.
func (m *MouseCornerMonitor) Run(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	consecutive := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if isAtCorner() {
				consecutive++
				if consecutive >= cornerThreshold {
					m.onUnlock()
					consecutive = 0
				}
			} else {
				consecutive = 0
			}
		}
	}
}

func isAtCorner() bool {
	var pt point
	//nolint:gosec // GetCursorPos expects a pointer to a Win32 POINT-compatible struct.
	ret, _, _ := getCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if ret == 0 {
		return false
	}
	return pt.X == 0 && pt.Y == 0
}
