package pty

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

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
