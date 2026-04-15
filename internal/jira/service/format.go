package service

import (
	"strings"
	"time"
)

// ParseTime tries common ISO 8601 layouts and returns zero time on failure.
func ParseTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// formatDateStr parses a stored date string and returns a short human-readable form.
func formatDateStr(s string) string {
	t := ParseTime(s)
	if t.IsZero() {
		return s
	}
	return t.Format("Jan 2, 2006")
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

// plural returns "s" when n != 1.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
