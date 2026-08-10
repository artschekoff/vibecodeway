package github

import (
	"strings"
	"testing"
)

func TestPRReviewArgs(t *testing.T) {
	got := strings.Join(prReviewArgs("acme", "app", 7, "--approve", "LGTM"), " ")
	if got != "pr review 7 --repo acme/app --approve --body LGTM" {
		t.Fatalf("with body: got %q", got)
	}
	got = strings.Join(prReviewArgs("acme", "app", 7, "--approve", ""), " ")
	if got != "pr review 7 --repo acme/app --approve" {
		t.Fatalf("no body: got %q", got)
	}
	if strings.Contains(got, "--body") {
		t.Fatal("empty body must not add --body flag")
	}
}

func TestReviewRequestSearch(t *testing.T) {
	if got := reviewRequestedSearch("@me"); got != "review-requested:@me draft:false" {
		t.Fatalf("got %q", got)
	}
	if got := reviewRequestedSearch("octocat"); got != "review-requested:octocat draft:false" {
		t.Fatalf("got %q", got)
	}
}

func TestAuthoredSearch(t *testing.T) {
	if got := authoredSearch("@me"); got != "author:@me draft:false" {
		t.Fatalf("got %q", got)
	}
	if got := authoredSearch("octocat"); got != "author:octocat draft:false" {
		t.Fatalf("got %q", got)
	}
}

func TestChangesRequestedSearch(t *testing.T) {
	if got := changesRequestedSearch("@me"); got != "reviewed-by:@me review:changes_requested draft:false" {
		t.Fatalf("got %q", got)
	}
	if got := changesRequestedSearch("octocat"); got != "reviewed-by:octocat review:changes_requested draft:false" {
		t.Fatalf("got %q", got)
	}
}
