package pty

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefaultShell(t *testing.T) {
	name, _ := DefaultShell()
	require.NotEmpty(t, name)
}

// Start runs a command in a PTY, streams its output via onData, and calls
// onExit when it finishes.
func TestManager_StartStreamsOutputAndExits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix shell command; ConPTY verified manually")
	}
	m := NewManager()
	t.Cleanup(m.CloseAll)

	var mu sync.Mutex
	var out string
	exited := make(chan error, 1)

	id, err := m.Start(
		t.TempDir(), "/bin/sh", []string{"-c", "printf hello_pty && exit 0"},
		func(_, chunk string) { mu.Lock(); out += chunk; mu.Unlock() },
		func(_ string, err error) { exited <- err },
	)
	require.NoError(t, err)
	require.NotEmpty(t, id)

	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Fatal("process did not exit in time")
	}
	mu.Lock()
	defer mu.Unlock()
	require.Contains(t, out, "hello_pty")
}

func TestManager_WriteAndClose(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix shell command")
	}
	m := NewManager()
	t.Cleanup(m.CloseAll)

	var mu sync.Mutex
	var out string
	id, err := m.Start(
		t.TempDir(), "/bin/sh", []string{"-i"},
		func(_, chunk string) { mu.Lock(); out += chunk; mu.Unlock() },
		func(string, error) {},
	)
	require.NoError(t, err)

	require.NoError(t, m.Write(id, "echo write_marker\n"))
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(out) > 0 && containsMarker(out, "write_marker")
	}, 5*time.Second, 50*time.Millisecond)

	require.NoError(t, m.Resize(id, 100, 40))
	require.NoError(t, m.Close(id))
	require.Error(t, m.Write(id, "x")) // closed session -> error
}

func containsMarker(s, marker string) bool {
	return len(s) >= len(marker) && (indexOf(s, marker) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
