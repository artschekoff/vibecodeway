package main

import (
	"bufio"
	"os/exec"
	"strings"
	"testing"
)

// TestCheckoutStreamPipe verifies bufio.Scanner reads command output line-by-line
// the same way openWithCheckout does. Guards against a scanner buffer regression.
func TestCheckoutStreamPipe(t *testing.T) {
	cmd := exec.Command("sh", "-c", "printf 'a\\nb\\nc\\n'")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	var got []string
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		got = append(got, sc.Text())
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if strings.Join(got, ",") != "a,b,c" {
		t.Fatalf("expected a,b,c got %v", got)
	}
}
