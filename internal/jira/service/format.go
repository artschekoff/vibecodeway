package service

import (
	"strings"
	"time"
)

// ParseTime tries common ISO 8601 layouts and returns zero time on failure.
//
// Jira's REST API (and acli, which just relays it) actually emits timestamps
// like "2024-01-15T10:30:00.000+0000" — a millisecond fraction plus a zone
// offset with no colon. That does NOT match time.RFC3339
// ("2006-01-02T15:04:05Z07:00", which requires "+00:00" or "Z"), so it needs
// its own layout; Go's fractional-seconds parsing is lenient enough that the
// same layout also accepts the offset-only form without ".000".
func ParseTime(s string) time.Time {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// FormatDateTime parses a stored date string and returns "YYYY-MM-DD HH:MM".
func FormatDateTime(s string) string {
	t := ParseTime(s)
	if t.IsZero() {
		return s
	}
	return t.Format("2006-01-02 15:04")
}

// indent prepends each non-empty line of text with the given prefix.
func indent(text, prefix string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = prefix + l
		}
	}
	return strings.Join(lines, "\n")
}
