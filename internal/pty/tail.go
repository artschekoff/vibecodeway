package pty

import "sync"

// sessionTailCapacity bounds how much recent PTY output a session keeps
// server-side. The frontend xterm instance owns the real scrollback; this
// exists only so server-side consumers (e.g. fallback code-tour generation,
// which reads "the tail of the terminal transcript") can see roughly the
// last ~32KB of what happened without the app re-reading from xterm.
const sessionTailCapacity = 32 * 1024

// tailBuffer is a fixed-capacity byte tail: appended output beyond capacity
// drops the oldest bytes, so String always returns (at most) the last
// capacity bytes ever written. Safe for concurrent use.
type tailBuffer struct {
	mu  sync.Mutex
	cap int
	buf []byte
}

func newTailBuffer(capacity int) *tailBuffer {
	return &tailBuffer{cap: capacity}
}

// Write appends p, trimming from the front if the result would exceed
// capacity. A single write larger than capacity keeps only its own tail.
func (t *tailBuffer) Write(p []byte) {
	if len(p) == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(p) >= t.cap {
		t.buf = append([]byte(nil), p[len(p)-t.cap:]...)
		return
	}
	t.buf = append(t.buf, p...)
	if excess := len(t.buf) - t.cap; excess > 0 {
		t.buf = append([]byte(nil), t.buf[excess:]...)
	}
}

// String returns the buffered tail as a string.
func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}
