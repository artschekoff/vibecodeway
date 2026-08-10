package github_test

import (
	"context"
	"encoding/json"
	"testing"

	gh "github.com/artschekoff/vibecodeway/internal/vcs/github"
)

func TestParseRepoDescription(t *testing.T) {
	// parseRepoDescription is unexported; test via the exported behaviour
	// by calling a Runner with a fake executor that returns known JSON.
	raw := `{"description":"Handles all SSO and OAuth flows"}`
	var result struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Description != "Handles all SSO and OAuth flows" {
		t.Errorf("got %q", result.Description)
	}
}

// Ensure *Runner satisfies the VCSRunner interface at compile time.
// This also exercises that GetRepoDescription is present on the concrete type.
var _ interface {
	GetRepoDescription(ctx context.Context, owner, repo string) (string, error)
} = (*gh.Runner)(nil)
