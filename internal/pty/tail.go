package pty

import (
	"sync"
	"unicode/utf8"
)

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
//
// Front-eviction advances the cut point forward to the next UTF-8 rune
// boundary (mirroring manager.go's splitTrailingRune) so a byte-offset cut
// never lands mid-rune: String() must never hand back a torn leading rune,
// since it gets JSON-encoded for callers (e.g. fallback tour generation)
// that expect valid text.
func (t *tailBuffer) Write(p []byte) {
	if len(p) == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(p) >= t.cap {
		cut := advanceToRuneStart(p, len(p)-t.cap)
		t.buf = append([]byte(nil), p[cut:]...)
		return
	}
	t.buf = append(t.buf, p...)
	if excess := len(t.buf) - t.cap; excess > 0 {
		cut := advanceToRuneStart(t.buf, excess)
		t.buf = append([]byte(nil), t.buf[cut:]...)
	}
}

// advanceToRuneStart returns the first index >= start in b that begins a
// UTF-8 rune, or len(b) if b has no rune start at or after start (e.g. start
// is already at the end, or the trailing bytes are all continuation bytes).
func advanceToRuneStart(b []byte, start int) int {
	for i := start; i < len(b); i++ {
		if utf8.RuneStart(b[i]) {
			return i
		}
	}
	return len(b)
}

// String returns the buffered tail as a string.
func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}
