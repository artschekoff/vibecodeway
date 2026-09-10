package pty

import (
	"runtime"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

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

// A PTY read ends at an arbitrary byte offset, so a multi-byte rune (Cyrillic,
// box-drawing, em dash) can straddle two reads. Emitting the halves separately
// makes the frontend render replacement chars, which also widens the line and
// shifts every column after it.
func TestSplitTrailingRune(t *testing.T) {
	const cyr = "\xd0\xb5" // 'е', 2 bytes

	complete, partial := splitTrailingRune([]byte("ok" + cyr))
	require.Equal(t, "ok"+cyr, string(complete))
	require.Empty(t, partial)

	complete, partial = splitTrailingRune([]byte("ok" + cyr[:1]))
	require.Equal(t, "ok", string(complete))
	require.Equal(t, cyr[:1], string(partial))

	// Complete but invalid encoding is emitted, not held back forever.
	complete, partial = splitTrailingRune([]byte("ok\xc3\x28"))
	require.Equal(t, "ok\xc3\x28", string(complete))
	require.Empty(t, partial)
}

// Every chunk handed to onData must be valid UTF-8 on its own: the Wails event
// bridge JSON-encodes it, and encoding/json turns stray bytes into U+FFFD.
func TestManager_ChunksNeverSplitARune(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix shell command")
	}
	m := NewManager()
	t.Cleanup(m.CloseAll)

	var mu sync.Mutex
	var chunks []string
	exited := make(chan error, 1)

	// One big write, consumed slowly: the tty queue backs up and starts handing
	// out fixed-size chunks that land mid-rune. A reader that keeps up gets the
	// writer's own line-sized writes and never hits the boundary.
	_, err := m.Start(
		t.TempDir(), "/bin/sh",
		[]string{"-c", "printf 'поддерживаемых фильтров — таблицей\\n%.0s' $(seq 1 400)"},
		func(_, chunk string) {
			time.Sleep(15 * time.Millisecond)
			mu.Lock()
			chunks = append(chunks, chunk)
			mu.Unlock()
		},
		func(_ string, err error) { exited <- err },
	)
	require.NoError(t, err)

	select {
	case <-exited:
	case <-time.After(10 * time.Second):
		t.Fatal("process did not exit in time")
	}
	mu.Lock()
	defer mu.Unlock()
	for i, c := range chunks {
		require.True(t, utf8.ValidString(c), "chunk %d is not valid UTF-8", i)
	}
}
