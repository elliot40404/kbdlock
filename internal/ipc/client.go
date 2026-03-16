package ipc

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
)

const (
	pipeName    = `\\.\pipe\kbdlock`
	dialTimeout = 2 * time.Second
	cmdTimeout  = 5 * time.Second
	maxRetries  = 5
	retryDelay  = 500 * time.Millisecond
)

// Client communicates with the sentinel over a named pipe.
type Client struct {
	mu   sync.Mutex
	conn net.Conn
}

// Connect opens the named pipe with retries.
func Connect() (*Client, error) {
	var conn net.Conn
	var err error

	for range maxRetries {
		timeout := dialTimeout
		conn, err = winio.DialPipe(pipeName, &timeout)
		if err == nil {
			return &Client{conn: conn}, nil
		}
		time.Sleep(retryDelay)
	}
	return nil, fmt.Errorf("connect to sentinel pipe after %d retries: %w", maxRetries, err)
}

// Close closes the pipe connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Send sends a command and returns the response line.
func (c *Client) Send(cmd string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return "", fmt.Errorf("not connected")
	}

	if err := c.conn.SetDeadline(time.Now().Add(cmdTimeout)); err != nil {
		return "", fmt.Errorf("set deadline for %q: %w", cmd, err)
	}

	_, err := fmt.Fprintf(c.conn, "%s\n", cmd)
	if err != nil {
		return "", fmt.Errorf("send %q: %w", cmd, err)
	}

	reader := bufio.NewReader(c.conn)
	resp, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read response for %q: %w", cmd, err)
	}
	return strings.TrimSpace(resp), nil
}

// Lock sends the LOCK command.
func (c *Client) Lock() error {
	resp, err := c.Send("LOCK")
	if err != nil {
		return err
	}
	if resp != "OK" {
		return fmt.Errorf("unexpected response: %s", resp)
	}
	return nil
}

// Unlock sends the UNLOCK command.
func (c *Client) Unlock() error {
	resp, err := c.Send("UNLOCK")
	if err != nil {
		return err
	}
	if resp != "OK" {
		return fmt.Errorf("unexpected response: %s", resp)
	}
	return nil
}

// Ping sends the PING command.
func (c *Client) Ping() error {
	resp, err := c.Send("PING")
	if err != nil {
		return err
	}
	if resp != "PONG" {
		return fmt.Errorf("unexpected ping response: %s", resp)
	}
	return nil
}

// Status queries the lock state.
func (c *Client) Status() (bool, error) {
	resp, err := c.Send("STATUS")
	if err != nil {
		return false, err
	}
	switch resp {
	case "LOCKED":
		return true, nil
	case "UNLOCKED":
		return false, nil
	default:
		return false, fmt.Errorf("unexpected status: %s", resp)
	}
}

// SetCombo sends the SET_COMBO command with VK codes.
func (c *Client) SetCombo(vkCodes []uint32) error {
	parts := make([]string, len(vkCodes))
	for i, vk := range vkCodes {
		parts[i] = fmt.Sprintf("%d", vk)
	}
	cmd := "SET_COMBO " + strings.Join(parts, " ")
	resp, err := c.Send(cmd)
	if err != nil {
		return err
	}
	if resp != "OK" {
		return fmt.Errorf("set combo failed: %s", resp)
	}
	return nil
}

// Quit sends the QUIT command.
func (c *Client) Quit() error {
	_, _ = c.Send("QUIT")
	return nil
}
