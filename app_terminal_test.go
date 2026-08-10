package main

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/artschekoff/vibecodeway/internal/pty"
	"github.com/stretchr/testify/require"
)

func TestTerminalStart_streamsData(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix shell")
	}
	var mu sync.Mutex
	events := map[string]string{}
	app := &App{
		ptyMgr: pty.NewManager(),
	}
	app.emit = func(name string, data ...interface{}) {
		mu.Lock()
		defer mu.Unlock()
		if len(data) > 0 {
			if s, ok := data[0].(string); ok {
				events[name] += s
			}
		} else {
			events[name] += ""
		}
	}
	t.Cleanup(app.ptyMgr.CloseAll)

	id, err := app.TerminalStart(t.TempDir())
	require.NoError(t, err)
	require.NotEmpty(t, id)

	require.NoError(t, app.TerminalWrite(id, "echo term_marker\n"))
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return contains(events["term:data:"+id], "term_marker")
	}, 5*time.Second, 50*time.Millisecond)

	require.NoError(t, app.TerminalResize(id, 80, 24))
	require.NoError(t, app.TerminalClose(id))
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
