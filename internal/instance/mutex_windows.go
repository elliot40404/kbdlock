package instance

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32     = syscall.NewLazyDLL("kernel32.dll")
	createMutex  = kernel32.NewProc("CreateMutexW")
	releaseMutex = kernel32.NewProc("ReleaseMutex")
	closeHandle  = kernel32.NewProc("CloseHandle")
)

const errorAlreadyExists = 183

// ErrAlreadyRunning indicates another controller instance is already active.
var ErrAlreadyRunning = errors.New("kbdlock is already running")

// Guard holds the app-level single-instance mutex.
type Guard struct {
	handle syscall.Handle
}

// AcquireController acquires the controller single-instance mutex.
func AcquireController() (*Guard, error) {
	name, err := controllerMutexName()
	if err != nil {
		return nil, err
	}

	ptr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, fmt.Errorf("encode mutex name: %w", err)
	}

	//nolint:gosec // Win32 CreateMutexW requires passing the UTF-16 name pointer as uintptr.
	handle, _, callErr := createMutex.Call(0, 1, uintptr(unsafe.Pointer(ptr)))
	if handle == 0 {
		return nil, fmt.Errorf("create controller mutex: %w", callErr)
	}

	var lastErr syscall.Errno
	ok := errors.As(callErr, &lastErr)
	if ok && lastErr == errorAlreadyExists {
		_, _, _ = closeHandle.Call(handle)
		return nil, ErrAlreadyRunning
	}

	return &Guard{handle: syscall.Handle(handle)}, nil
}

// IsControllerRunning reports whether another controller instance currently owns
// the app-level single-instance mutex.
func IsControllerRunning() (bool, error) {
	guard, err := AcquireController()
	if errors.Is(err, ErrAlreadyRunning) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if guard != nil {
		_ = guard.Close()
	}
	return false, nil
}

// Close releases the mutex handle.
func (g *Guard) Close() error {
	if g == nil || g.handle == 0 {
		return nil
	}

	handle := g.handle
	g.handle = 0
	_, _, _ = releaseMutex.Call(uintptr(handle))
	if ret, _, err := closeHandle.Call(uintptr(handle)); ret == 0 {
		return fmt.Errorf("close controller mutex: %w", err)
	}
	return nil
}

func controllerMutexName() (string, error) {
	userName := os.Getenv("USERNAME")
	if userName == "" {
		return "", errors.New("USERNAME environment variable not set")
	}

	userName = strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z':
			return r
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, userName)

	return `Local\kbdlock-controller-` + userName, nil
}
