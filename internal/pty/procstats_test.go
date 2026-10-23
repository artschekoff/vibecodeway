package pty

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Tree: app(100) → shellA(200) → agent(300) → worker(400); shellB(500) idle.
const psFixture = `  100     1  1.0  1000 /Applications/App.app/Contents/MacOS/app
  200   100  0.0   500 /bin/zsh
  300   200 55.5 80000 /usr/local/bin/claude
  400   300 10.0 20000 node with spaces
  500   100  0.0   400 /bin/zsh
  999     1  3.0   100 unrelated
garbage line
`

func TestBuildStatsAggregatesSubtrees(t *testing.T) {
	stats := buildStats(parsePS(psFixture), 100, map[string]int{"a": 200, "b": 500})

	assert.Equal(t, 100, stats.App.Pid)
	assert.InDelta(t, 66.5, stats.App.CPU, 0.001) // 1.0+0.0+55.5+10.0+0.0
	assert.Equal(t, int64((1000+500+80000+20000+400)*1024), stats.App.RSSBytes)
	assert.Equal(t, 5, stats.App.Procs)
	assert.Equal(t, "claude", stats.App.TopComm)

	byID := map[string]SessionStat{}
	for _, s := range stats.Sessions {
		byID[s.SessionID] = s
	}
	a := byID["a"]
	assert.InDelta(t, 65.5, a.CPU, 0.001)
	assert.Equal(t, 3, a.Procs)
	assert.Equal(t, "claude", a.TopComm)
	assert.Equal(t, int64((500+80000+20000)*1024), a.RSSBytes)

	b := byID["b"]
	assert.InDelta(t, 0.0, b.CPU, 0.001)
	assert.Equal(t, 1, b.Procs)
	assert.Equal(t, "zsh", b.TopComm) // idle subtree still names its only process
}
