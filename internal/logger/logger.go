package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/elliot40404/kbdlock/internal/config"
)

const (
	maxLogSize = 5 * 1024 * 1024 // 5 MB
	logFile    = "kbdlock.log"
)

// Logger writes timestamped messages to a log file.
type Logger struct {
	mu   sync.Mutex
	file *os.File
}

// New creates a logger writing to %APPDATA%\kbdlock\kbdlock.log.
// Rotates the file if it exceeds 5MB.
func New() (*Logger, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}

	path := filepath.Join(dir, logFile)
	rotate(path)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &Logger{file: f}, nil
}

// Info logs an informational message.
func (l *Logger) Info(format string, args ...any) {
	l.write("INFO", format, args...)
}

// Error logs an error message.
func (l *Logger) Error(format string, args ...any) {
	l.write("ERROR", format, args...)
}

// Close closes the log file.
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_ = l.file.Close()
	}
}

func (l *Logger) write(level, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] %s: %s\n", ts, level, msg)

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_, _ = l.file.WriteString(line)
	}
}

// rotate renames the log to .log.old if it exceeds maxLogSize.
func rotate(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxLogSize {
		return
	}
	_ = os.Rename(path, path+".old")
}
