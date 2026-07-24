// Package pty manages interactive pseudo-terminal sessions (Unix PTY / Windows
// ConPTY) for the embedded editor terminal.
package pty

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"

	gopty "github.com/aymanbagabas/go-pty"
	"github.com/google/uuid"
)

// DefaultShell returns the interactive login shell for the current OS.
func DefaultShell() (string, []string) {
	if runtime.GOOS == "windows" {
		if sh := os.Getenv("COMSPEC"); sh != "" {
			return sh, nil
		}
		return "powershell.exe", nil
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh, []string{"-l"}
	}
	return "/bin/sh", []string{"-l"}
}

type session struct {
	pty gopty.Pty
	cmd *gopty.Cmd
}

// Manager owns live PTY sessions keyed by an opaque session id.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*session
}

func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*session)}
}

// Start launches name+args in a new PTY rooted at cwd. onData receives every
// output chunk; onExit fires once when the process ends. Returns the session id.
func (m *Manager) Start(cwd, name string, args []string, onData func(id, chunk string), onExit func(id string, err error)) (string, error) {
	p, err := gopty.New()
	if err != nil {
		return "", fmt.Errorf("open pty: %w", err)
	}
	c := p.Command(name, args...)
	if cwd != "" {
		c.Dir = cwd
	}
	if err := c.Start(); err != nil {
		_ = p.Close()
		return "", fmt.Errorf("start pty command: %w", err)
	}

	// id is generated before the goroutines launch, so both callbacks always
	// receive the correct id (no capture race for callers).
	id := uuid.New().String()
	m.mu.Lock()
	m.sessions[id] = &session{pty: p, cmd: c}
	m.mu.Unlock()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, rerr := p.Read(buf)
			if n > 0 && onData != nil {
				onData(id, string(buf[:n]))
			}
			if rerr != nil {
				break
			}
		}
	}()
	go func() {
		werr := c.Wait()
		m.mu.Lock()
		delete(m.sessions, id)
		m.mu.Unlock()
		_ = p.Close()
		if onExit != nil {
			onExit(id, werr)
		}
	}()

	return id, nil
}

func (m *Manager) get(id string) (*session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("pty session not found")
	}
	return s, nil
}

func (m *Manager) Write(id, data string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	_, err = io.WriteString(s.pty, data)
	return err
}

func (m *Manager) Resize(id string, cols, rows int) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	return s.pty.Resize(cols, rows)
}

func (m *Manager) Close(id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if !ok {
		return nil
	}
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return s.pty.Close()
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.Close(id)
	}
}
