package appconsole

import (
	"syscall"
	"unsafe"
)

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	user32                = syscall.NewLazyDLL("user32.dll")
	getConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	getConsoleWindow      = kernel32.NewProc("GetConsoleWindow")
	freeConsole           = kernel32.NewProc("FreeConsole")
	showWindow            = user32.NewProc("ShowWindow")
)

const (
	swHide = 0
)

// HasAttachedTerminal reports whether the current process is attached to a
// terminal that existed before it started.
func HasAttachedTerminal() bool {
	return consoleProcessCount() > 1
}

// HideIfStandalone detaches the process from its own console window when it
// was launched without an existing terminal attached, such as from Explorer.
func HideIfStandalone() {
	if HasAttachedTerminal() || consoleProcessCount() == 0 {
		return
	}

	if hwnd, _, _ := getConsoleWindow.Call(); hwnd != 0 {
		_, _, _ = showWindow.Call(hwnd, swHide)
	}
	_, _, _ = freeConsole.Call()
}

func consoleProcessCount() uint32 {
	pids := make([]uint32, 8)
	//nolint:gosec // Win32 API requires a pointer to the process ID buffer.
	count, _, _ := getConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	return uint32(count)
}
