package main

import "testing"

func TestSplitOwnerRepo(t *testing.T) {
	owner, repo, err := splitOwnerRepo("acme/frontend")
	if err != nil || owner != "acme" || repo != "frontend" {
		t.Fatalf("splitOwnerRepo = %q, %q, %v", owner, repo, err)
	}
	for _, bad := range []string{"", "frontend", "/frontend", "acme/", "/"} {
		if _, _, err := splitOwnerRepo(bad); err == nil {
			t.Errorf("splitOwnerRepo(%q) must error", bad)
		}
	}
}

func TestSplitOwnerRepo_KeepsNestedPathAsRepoName(t *testing.T) {
	// SplitN(…, 2) means a GitLab-style group/sub/repo keeps everything after
	// the first slash as the name — documented behaviour, not a bug.
	owner, repo, err := splitOwnerRepo("group/sub/repo")
	if err != nil || owner != "group" || repo != "sub/repo" {
		t.Fatalf("splitOwnerRepo = %q, %q, %v", owner, repo, err)
	}
}
