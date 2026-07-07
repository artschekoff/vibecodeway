package main

import (
	"bufio"
	"os/exec"
	"testing"
)

// TestCheckoutStreamPipe verifies bufio.Scanner reads command output line-by-line
// with the correct buffer configuration. Guards against a scanner buffer regression
// by ensuring a 200KB line can be read in full only if sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) is configured.
func TestCheckoutStreamPipe(t *testing.T) {
	// Generate a line with ~200,000 characters followed by newline.
	// This exceeds the default bufio.Scanner max token size of 64KB,
	// so the test only passes if the scanner is configured with a larger buffer.
	cmd := exec.Command("sh", "-c", "printf '%*s\\n' 200000 x")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	var got []string
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) // Match production code
	for sc.Scan() {
		got = append(got, sc.Text())
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if sc.Err() != nil {
		t.Fatalf("scanner error: %v", sc.Err())
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 line, got %d", len(got))
	}
	if len(got[0]) != 200000 {
		t.Fatalf("expected line length 200000, got %d", len(got[0]))
	}
}
