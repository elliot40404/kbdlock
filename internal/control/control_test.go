package control

import (
	"sync"
	"testing"
)

type fakeHandler struct {
	mu          sync.Mutex
	pid         int
	lockCalls   int
	unlockCalls int
	stopCalls   int
	lockErr     error
	unlockErr   error
}

func (h *fakeHandler) LockCommand() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lockCalls++
	return h.lockErr
}

func (h *fakeHandler) UnlockCommand() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.unlockCalls++
	return h.unlockErr
}

func (h *fakeHandler) StopCommand() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stopCalls++
}

func (h *fakeHandler) PID() int {
	return h.pid
}

func TestProcessCommandLockUnlockStopAndPID(t *testing.T) {
	handler := &fakeHandler{pid: 1234}
	server := &Server{handler: handler}

	for _, tc := range []struct {
		cmd      string
		wantResp string
		wantStop bool
	}{
		{cmd: "LOCK", wantResp: "OK"},
		{cmd: "UNLOCK", wantResp: "OK"},
		{cmd: "PID", wantResp: "PID 1234"},
		{cmd: "STOP", wantResp: "OK", wantStop: true},
	} {
		gotResp, gotStop := server.processCommand(tc.cmd)
		if gotResp != tc.wantResp {
			t.Fatalf("processCommand(%q) resp = %q, want %q", tc.cmd, gotResp, tc.wantResp)
		}
		if gotStop != tc.wantStop {
			t.Fatalf("processCommand(%q) stop = %v, want %v", tc.cmd, gotStop, tc.wantStop)
		}
	}
}

func TestProcessCommandReportsHandlerErrors(t *testing.T) {
	handler := &fakeHandler{lockErr: ErrNotRunning}
	server := &Server{handler: handler}

	resp, stop := server.processCommand("LOCK")
	if resp != "ERROR "+ErrNotRunning.Error() {
		t.Fatalf("processCommand(LOCK) resp = %q", resp)
	}
	if stop {
		t.Fatal("processCommand(LOCK) stop = true, want false")
	}
}

func TestProcessCommandUnknown(t *testing.T) {
	server := &Server{handler: &fakeHandler{}}

	resp, stop := server.processCommand("BOGUS")
	if resp != "ERROR unknown command" {
		t.Fatalf("processCommand(BOGUS) resp = %q", resp)
	}
	if stop {
		t.Fatal("processCommand(BOGUS) stop = true, want false")
	}
}
