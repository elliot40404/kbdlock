package sentinel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/elliot40404/kbdlock/internal/ipc"
	"github.com/elliot40404/kbdlock/internal/logger"
)

const (
	sentinelExe  = "kbdlock-hook.exe"
	pingInterval = 5 * time.Second
	shutdownWait = 3 * time.Second
	restartDelay = 2 * time.Second
)

// Manager handles sentinel process lifecycle and watchdog pings.
type Manager struct {
	log    *logger.Logger
	mu     sync.Mutex
	cmd    *exec.Cmd
	client *ipc.Client
	locked bool // desired lock state for restoration after restart
	cancel context.CancelFunc
}

// New creates a sentinel manager.
func New(log *logger.Logger) *Manager {
	return &Manager{log: log}
}

// Start launches the sentinel process.
func (m *Manager) Start() error {
	exePath, err := m.sentinelPath()
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	//nolint:gosec // exePath comes from our own extracted embedded sentinel binary.
	m.cmd = exec.Command(exePath)
	m.cmd.Stdout = os.Stdout
	m.cmd.Stderr = os.Stderr
	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("start sentinel: %w", err)
	}

	m.log.Info("sentinel started (pid %d)", m.cmd.Process.Pid)
	return nil
}

// Connect establishes the IPC connection to the sentinel.
func (m *Manager) Connect() (*ipc.Client, error) {
	client, err := ipc.Connect()
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.client = client
	m.mu.Unlock()

	m.log.Info("connected to sentinel pipe")
	return client, nil
}

// StartWatchdog starts a goroutine that pings the sentinel periodically
// and restarts it if it becomes unresponsive.
func (m *Manager) StartWatchdog(ctx context.Context, onRestart func(*ipc.Client)) {
	watchCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.cancel = cancel
	m.mu.Unlock()

	go m.watchdog(watchCtx, onRestart)
}

// SetDesiredLock records the desired lock state for restoration after restart.
func (m *Manager) SetDesiredLock(locked bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.locked = locked
}

// Stop shuts down the sentinel gracefully.
func (m *Manager) Stop() {
	m.mu.Lock()
	cancel := m.cancel
	client := m.client
	cmd := m.cmd
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	// Try graceful shutdown via QUIT.
	if client != nil {
		if err := client.Quit(); err != nil {
			m.log.Error("sentinel quit command failed: %v", err)
		}
		if err := client.Close(); err != nil {
			m.log.Error("sentinel pipe close failed: %v", err)
		}
	}

	if cmd != nil && cmd.Process != nil {
		waitErrCh := make(chan error, 1)
		go func() {
			waitErrCh <- cmd.Wait()
		}()

		select {
		case err := <-waitErrCh:
			if err != nil {
				m.log.Error("sentinel wait failed: %v", err)
			}
		case <-time.After(shutdownWait):
			if err := cmd.Process.Kill(); err != nil {
				m.log.Error("sentinel kill failed: %v", err)
			}
			if err := <-waitErrCh; err != nil {
				m.log.Error("sentinel wait after kill failed: %v", err)
			}
		}
	}

	m.log.Info("sentinel stopped")
}

func (m *Manager) watchdog(ctx context.Context, onRestart func(*ipc.Client)) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.mu.Lock()
			client := m.client
			m.mu.Unlock()

			if client == nil {
				continue
			}

			if err := client.Ping(); err != nil {
				m.log.Error("sentinel ping failed: %v, restarting...", err)
				m.restart(ctx, onRestart)
			}
		}
	}
}

func (m *Manager) restart(ctx context.Context, onRestart func(*ipc.Client)) {
	m.mu.Lock()
	if m.client != nil {
		if err := m.client.Close(); err != nil {
			m.log.Error("sentinel pipe close before restart failed: %v", err)
		}
		m.client = nil
	}
	m.mu.Unlock()

	time.Sleep(restartDelay)

	if ctx.Err() != nil {
		return
	}

	if err := m.Start(); err != nil {
		m.log.Error("failed to restart sentinel: %v", err)
		return
	}

	client, err := m.Connect()
	if err != nil {
		m.log.Error("failed to reconnect to sentinel: %v", err)
		return
	}

	// Restore desired lock state.
	m.mu.Lock()
	wasLocked := m.locked
	m.mu.Unlock()

	if wasLocked {
		if err := client.Lock(); err != nil {
			m.log.Error("failed to restore lock state: %v", err)
		}
	}

	if onRestart != nil {
		onRestart(client)
	}
}

// sentinelPath extracts the embedded sentinel exe and returns its path.
func (m *Manager) sentinelPath() (string, error) {
	return ExtractSentinel(m.log)
}
