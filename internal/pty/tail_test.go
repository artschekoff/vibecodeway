package pty

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Writing chunks that together exceed capacity keeps only the most recently
// written bytes — "last N bytes win".
func TestTailBuffer_KeepsOnlyLastCapacityBytes(t *testing.T) {
	tb := newTailBuffer(10)

	tb.Write([]byte("0123456789")) // exactly capacity
	assert.Equal(t, "0123456789", tb.String())

	tb.Write([]byte("ABC")) // pushes out the oldest 3 bytes
	assert.Equal(t, "3456789ABC", tb.String())

	tb.Write([]byte("XYZXYZXYZXYZ")) // single write bigger than capacity
	assert.Equal(t, "ZXYZXYZXYZ", tb.String())
}

// A single write larger than capacity must not evict mid-rune: the cut point
// has to land on the next UTF-8 rune boundary, never inside a multi-byte
// rune's continuation bytes.
func TestTailBuffer_SingleWriteEvictionNeverSplitsARune(t *testing.T) {
	const cyr = "\xd0\xb5" // 'е', 2 bytes; second byte is a UTF-8 continuation byte
	p := []byte("ab" + cyr + "cd")

	// cap=3 forces the naive cut (len(p)-cap = 3) to land on cyr's
	// continuation byte; the fix must advance past it to the next full rune.
	tb := newTailBuffer(3)
	tb.Write(p)

	out := tb.String()
	require.True(t, utf8.ValidString(out), "tail must never contain a torn leading rune: %q", out)
	assert.Equal(t, "cd", out)
}

// Front-eviction that happens while trimming previously-accumulated bytes
// (the "append then trim excess" path, as opposed to a single oversized
// write) must also respect rune boundaries.
func TestTailBuffer_AppendEvictionNeverSplitsARune(t *testing.T) {
	const cyr = "\xd0\xb5" // 'е', 2 bytes

	tb := newTailBuffer(7)
	tb.Write([]byte("ab"))            // buf: "ab" (2 bytes, under capacity)
	tb.Write([]byte(cyr + cyr + cyr)) // buf: "ab" + cyr*3 = 8 bytes, excess=1, trims to "b"+cyr*3 (7 bytes)
	tb.Write([]byte(cyr))             // buf: "b"+cyr*3+cyr = 9 bytes, excess=2 lands mid-rune

	out := tb.String()
	require.True(t, utf8.ValidString(out), "tail must never contain a torn leading rune: %q", out)
	assert.Equal(t, cyr+cyr+cyr, out)
}

func TestTailBuffer_UnderCapacityReturnsEverythingWritten(t *testing.T) {
	tb := newTailBuffer(1024)
	tb.Write([]byte("hello "))
	tb.Write([]byte("world"))
	assert.Equal(t, "hello world", tb.String())
}

// Many goroutines writing concurrently must not race or panic; run with -race.
func TestTailBuffer_ConcurrentWritesDoNotRace(t *testing.T) {
	tb := newTailBuffer(64)

	var wg sync.WaitGroup
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				tb.Write([]byte(fmt.Sprintf("g%d-%d;", n, i)))
			}
		}(g)
	}
	wg.Wait()

	// No assertion on exact content (write order across goroutines is
	// nondeterministic) — this test's job is to prove -race finds nothing and
	// the buffer never exceeds its capacity.
	assert.LessOrEqual(t, len(tb.String()), 64)
}

func TestManager_TailOutputAndTailForCwd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix shell command; ConPTY verified manually")
	}
	m := NewManager()
	t.Cleanup(m.CloseAll)

	// Unknown session id / cwd: "" rather than a panic or error.
	assert.Equal(t, "", m.TailOutput("does-not-exist"))
	assert.Equal(t, "", m.TailForCwd("/nowhere"))

	cwd := t.TempDir()
	id, err := m.Start(cwd, "/bin/sh", []string{"-i"}, nil, func(string, error) {})
	require.NoError(t, err)

	require.NoError(t, m.Write(id, "echo tail_marker\n"))
	require.Eventually(t, func() bool {
		return strings.Contains(m.TailOutput(id), "tail_marker")
	}, 5*time.Second, 50*time.Millisecond)
	assert.Contains(t, m.TailForCwd(cwd), "tail_marker")

	require.NoError(t, m.Close(id))
	assert.Equal(t, "", m.TailOutput(id)) // gone once closed
	assert.Equal(t, "", m.TailForCwd(cwd))
}
