// Process-tree resource stats for PTY sessions: one `ps` snapshot, aggregated
// per session subtree. On-demand only — no background collection.
package pty

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// SessionStat aggregates one process subtree (a PTY session's shell and
// everything it spawned, or the app itself).
type SessionStat struct {
	SessionID string  `json:"sessionId"`
	Pid       int     `json:"pid"`
	CPU       float64 `json:"cpu"`
	RSSBytes  int64   `json:"rssBytes"`
	Procs     int     `json:"procs"`
	// TopComm is the basename of the busiest process in the subtree — for a
	// coding-agent tab this is the agent binary ("claude", "node", ...).
	TopComm string `json:"topComm"`
}

// ProcessStats is the Task Manager payload: the whole app plus every live
// PTY session.
type ProcessStats struct {
	App      SessionStat   `json:"app"`
	Sessions []SessionStat `json:"sessions"`
}

type procSample struct {
	pid, ppid int
	cpu       float64
	rssKb     int64
	comm      string
}

// parsePS parses `ps -axo pid=,ppid=,pcpu=,rss=,comm=` output. comm may
// contain spaces (full executable path), so it is the join of the tail fields.
func parsePS(out string) []procSample {
	var samples []procSample
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 5 {
			continue
		}
		pid, err1 := strconv.Atoi(f[0])
		ppid, err2 := strconv.Atoi(f[1])
		cpu, err3 := strconv.ParseFloat(f[2], 64)
		rss, err4 := strconv.ParseInt(f[3], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			continue
		}
		samples = append(samples, procSample{pid: pid, ppid: ppid, cpu: cpu, rssKb: rss, comm: strings.Join(f[4:], " ")})
	}
	return samples
}

// aggregateSubtree walks pid's descendant tree (pid included) and sums CPU/RSS.
func aggregateSubtree(children map[int][]procSample, byPid map[int]procSample, pid int) SessionStat {
	stat := SessionStat{Pid: pid}
	var topCPU float64 = -1
	var topRSS int64 = -1
	stack := []int{pid}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		s, ok := byPid[p]
		if ok {
			stat.CPU += s.cpu
			stat.RSSBytes += s.rssKb * 1024
			stat.Procs++
			if s.cpu > topCPU || (s.cpu == topCPU && s.rssKb > topRSS) {
				topCPU, topRSS = s.cpu, s.rssKb
				stat.TopComm = filepath.Base(s.comm)
			}
		}
		for _, c := range children[p] {
			stack = append(stack, c.pid)
		}
	}
	return stat
}

// buildStats aggregates a ps snapshot for the app pid and each session root.
func buildStats(samples []procSample, appPid int, sessionPids map[string]int) ProcessStats {
	byPid := make(map[int]procSample, len(samples))
	children := make(map[int][]procSample, len(samples))
	for _, s := range samples {
		byPid[s.pid] = s
		children[s.ppid] = append(children[s.ppid], s)
	}
	out := ProcessStats{App: aggregateSubtree(children, byPid, appPid)}
	for id, pid := range sessionPids {
		st := aggregateSubtree(children, byPid, pid)
		st.SessionID = id
		out.Sessions = append(out.Sessions, st)
	}
	return out
}

// Pids returns the root shell PID of every live session, keyed by session id.
func (m *Manager) Pids() map[string]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]int, len(m.sessions))
	for id, s := range m.sessions {
		if s.cmd != nil && s.cmd.Process != nil {
			out[id] = s.cmd.Process.Pid
		}
	}
	return out
}

// ProcessStats snapshots `ps` once and aggregates the app's own subtree plus
// every live PTY session's subtree. Unix-only; on failure returns the error.
func (m *Manager) ProcessStats() (ProcessStats, error) {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,pcpu=,rss=,comm=").Output()
	if err != nil {
		return ProcessStats{}, err
	}
	return buildStats(parsePS(string(out)), os.Getpid(), m.Pids()), nil
}
