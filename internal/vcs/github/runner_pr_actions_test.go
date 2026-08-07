package github

import "testing"

func TestReviewRequestSearch(t *testing.T) {
	if got := reviewRequestedSearch("@me"); got != "review-requested:@me draft:false" {
		t.Fatalf("got %q", got)
	}
	if got := reviewRequestedSearch("octocat"); got != "review-requested:octocat draft:false" {
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
