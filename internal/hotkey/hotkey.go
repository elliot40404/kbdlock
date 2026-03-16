package hotkey

import (
	"context"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	user32            = syscall.NewLazyDLL("user32.dll")
	registerHotKey    = user32.NewProc("RegisterHotKey")
	unregisterHotKey  = user32.NewProc("UnregisterHotKey")
	createWindowExW   = user32.NewProc("CreateWindowExW")
	destroyWindow     = user32.NewProc("DestroyWindow")
	getMessage        = user32.NewProc("GetMessageW")
	postThreadMessage = user32.NewProc("PostThreadMessageW")
	getThreadID       = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId")
)

const (
	wmHotkey = 0x0312
	wmQuit   = 0x0012
	hotkeyID = 1
)

type msg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// Listener listens for a global hotkey and calls onToggle when pressed.
type Listener struct {
	log interface {
		Info(string, ...any)
		Error(string, ...any)
	}
	modifiers uint32
	vk        uint32
	onToggle  func()
}

// New creates a hotkey listener.
func New(log interface {
	Info(string, ...any)
	Error(string, ...any)
}, modifiers, vk uint32, onToggle func(),
) *Listener {
	return &Listener{log: log, modifiers: modifiers, vk: vk, onToggle: onToggle}
}

// Run registers the hotkey and runs a message loop until ctx is cancelled.
// Must NOT be called from the main goroutine if it is already running a message loop.
func (l *Listener) Run(ctx context.Context) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	tid, _, _ := getThreadID.Call()

	// Create a hidden message-only window.
	className, _ := syscall.UTF16PtrFromString("kbdlock_hotkey")
	//nolint:gosec // Win32 API requires passing the UTF-16 pointer as uintptr.
	hwnd, _, _ := createWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		0, // no title
		0, // style
		0, 0, 0, 0,
		^uintptr(2), // HWND_MESSAGE = (HWND)-3, but we use 0 for simplicity
		0, 0, 0,
	)

	ret, _, err := registerHotKey.Call(hwnd, hotkeyID, uintptr(l.modifiers), uintptr(l.vk))
	if ret == 0 {
		l.log.Error("RegisterHotKey failed (combo may be taken): %v", err)
		if hwnd != 0 {
			_, _, _ = destroyWindow.Call(hwnd)
		}
		return
	}
	l.log.Info("global hotkey registered")

	// Cancel goroutine: post WM_QUIT to break the message loop.
	go func() {
		<-ctx.Done()
		_, _, _ = postThreadMessage.Call(tid, wmQuit, 0, 0)
	}()

	// Message loop.
	var m msg
	for {
		//nolint:gosec // Win32 message loop requires the struct pointer as uintptr.
		ret, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break // WM_QUIT or error
		}
		if m.Message == wmHotkey {
			l.onToggle()
		}
	}

	_, _, _ = unregisterHotKey.Call(hwnd, hotkeyID)
	if hwnd != 0 {
		_, _, _ = destroyWindow.Call(hwnd)
	}
	l.log.Info("global hotkey unregistered")
}
