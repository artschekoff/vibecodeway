package main

import (
	"testing"
)

func TestAgentToolCallRequiresYesForWrites(t *testing.T) {
	if err := guardWrite(true, false); err == nil {
		t.Fatal("write tool without --yes must error")
	}
	if err := guardWrite(true, true); err != nil {
		t.Fatalf("write tool with --yes must pass: %v", err)
	}
	if err := guardWrite(false, false); err != nil {
		t.Fatalf("read tool never needs --yes: %v", err)
	}
}

func TestAgentToolCallParsesArgs(t *testing.T) {
	args, err := parseToolArgs(`{"contextId":"abc","key":"k"}`)
	if err != nil || args["contextId"] != "abc" {
		t.Fatalf("parseToolArgs = %v, %v", args, err)
	}
	if _, err := parseToolArgs("not json"); err == nil {
		t.Fatalf("invalid JSON must error")
	}
	if got, _ := parseToolArgs(""); got == nil || len(got) != 0 {
		t.Fatalf("empty args must be empty non-nil map, got %v", got)
	}
}
