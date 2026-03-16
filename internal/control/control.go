package control

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/elliot40404/kbdlock/internal/instance"
	"golang.org/x/sys/windows"
)

const (
	pipePrefix     = `\\.\pipe\kbdlock-controller-`
	connectTimeout = 2 * time.Second
	commandTimeout = 5 * time.Second
	stopTimeout    = 3 * time.Second
)

var (
	ErrNotRunning         = errors.New("kbdlock is not running")
	ErrControlUnavailable = errors.New("kbdlock is running but does not support CLI control commands")
)

func StopTimeout() time.Duration {
	return stopTimeout
}

type handler interface {
	LockCommand() error
	UnlockCommand() error
	StopCommand()
	PID() int
}

type Server struct {
	listener  net.Listener
	handler   handler
	closeOnce sync.Once
}

func Listen(h handler) (*Server, error) {
	pipeName, err := pipeName()
	if err != nil {
		return nil, err
	}
	return listenAt(pipeName, h)
}

func listenAt(pipeName string, h handler) (*Server, error) {
	listener, err := winio.ListenPipe(pipeName, nil)
	if err != nil {
		return nil, fmt.Errorf("listen controller pipe: %w", err)
	}
	return &Server{listener: listener, handler: h}, nil
}

func (s *Server) Run() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			var errno windows.Errno
			if errors.As(err, &errno) && errno == windows.ERROR_NO_DATA {
				return
			}
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *Server) Close() error {
	var err error
	s.closeOnce.Do(func() {
		err = s.listener.Close()
	})
	return err
}

type Client struct {
	conn net.Conn
}

func Connect() (*Client, error) {
	pipeName, err := pipeName()
	if err != nil {
		return nil, err
	}
	return connectTo(pipeName)
}

func connectTo(pipeName string) (*Client, error) {
	timeout := connectTimeout
	conn, err := winio.DialPipe(pipeName, &timeout)
	if err == nil {
		return &Client{conn: conn}, nil
	}

	running, runningErr := instance.IsControllerRunning()
	if runningErr == nil && !running {
		return nil, ErrNotRunning
	}
	if runningErr != nil {
		return nil, fmt.Errorf("check running controller: %w", runningErr)
	}

	return nil, fmt.Errorf("%w: %w", ErrControlUnavailable, err)
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) Lock() error {
	return c.expectOK("LOCK")
}

func (c *Client) Unlock() error {
	return c.expectOK("UNLOCK")
}

func (c *Client) Stop() error {
	return c.expectOK("STOP")
}

func (c *Client) PID() (int, error) {
	resp, err := c.send("PID")
	if err != nil {
		return 0, err
	}
	var pid int
	if _, err := fmt.Sscanf(resp, "PID %d", &pid); err != nil {
		return 0, fmt.Errorf("parse PID response %q: %w", resp, err)
	}
	return pid, nil
}

func WaitForExitOrKill(pid int, timeout time.Duration) error {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return nil
		}
		return fmt.Errorf("open controller process %d: %w", pid, err)
	}
	defer func() {
		_ = windows.CloseHandle(handle)
	}()

	waitMS := uint32(timeout / time.Millisecond)
	switch status, waitErr := windows.WaitForSingleObject(handle, waitMS); status {
	case uint32(windows.WAIT_OBJECT_0):
		return nil
	case uint32(windows.WAIT_TIMEOUT):
		if termErr := windows.TerminateProcess(handle, 1); termErr != nil {
			return fmt.Errorf("terminate controller process %d: %w", pid, termErr)
		}
		if _, waitErr = windows.WaitForSingleObject(handle, waitMS); waitErr != nil {
			return fmt.Errorf("wait for terminated controller process %d: %w", pid, waitErr)
		}
		return nil
	default:
		if waitErr != nil {
			return fmt.Errorf("wait for controller process %d: %w", pid, waitErr)
		}
		return fmt.Errorf("wait for controller process %d returned status %d", pid, status)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer func() {
		_ = conn.Close()
	}()
	_ = conn.SetDeadline(time.Now().Add(commandTimeout))

	reader := bufio.NewReader(conn)
	cmd, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	cmd = strings.TrimSpace(cmd)

	resp, triggerStop := s.processCommand(cmd)
	//nolint:gosec // response is a plain protocol token written to a local named-pipe client.
	_, _ = fmt.Fprintf(conn, "%s\n", resp)
	if triggerStop {
		go s.handler.StopCommand()
	}
}

func (c *Client) send(cmd string) (string, error) {
	if c == nil || c.conn == nil {
		return "", ErrNotRunning
	}
	if err := c.conn.SetDeadline(time.Now().Add(commandTimeout)); err != nil {
		return "", fmt.Errorf("set deadline for %q: %w", cmd, err)
	}
	if _, err := fmt.Fprintf(c.conn, "%s\n", cmd); err != nil {
		return "", fmt.Errorf("send %q: %w", cmd, err)
	}

	resp, err := bufio.NewReader(c.conn).ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read response for %q: %w", cmd, err)
	}
	return strings.TrimSpace(resp), nil
}

func (c *Client) expectOK(cmd string) error {
	resp, err := c.send(cmd)
	if err != nil {
		return err
	}
	if resp == "OK" {
		return nil
	}
	if after, ok := strings.CutPrefix(resp, "ERROR "); ok {
		return errors.New(after)
	}
	return fmt.Errorf("unexpected response: %s", resp)
}

func respondOK(err error) string {
	if err != nil {
		return "ERROR " + err.Error()
	}
	return "OK"
}

func (s *Server) processCommand(cmd string) (string, bool) {
	switch cmd {
	case "LOCK":
		return respondOK(s.handler.LockCommand()), false
	case "UNLOCK":
		return respondOK(s.handler.UnlockCommand()), false
	case "STOP":
		return "OK", true
	case "PID":
		return fmt.Sprintf("PID %d", s.handler.PID()), false
	default:
		return "ERROR unknown command", false
	}
}

func pipeName() (string, error) {
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

	return pipePrefix + userName, nil
}
