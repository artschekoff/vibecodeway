package main

import (
	"os"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachReviewRemarksPicksNearestInlineComments(t *testing.T) {
	repo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "src"), 0o755))
	var body []byte
	for i := 1; i <= 200; i++ {
		line := "line\n"
		if i == 120 {
			line = "func handleReferrals(\n"
		}
		body = append(body, line...)
	}
	require.NoError(t, os.WriteFile(filepath.Join(repo, "src", "x.ts"), body, 0o644))

	path := "src/x.ts"
	near, far, other := 110, 10, 5
	comments := []vcsstore.PRComment{
		{Author: "bob", CommentType: "review_comment", Path: &path, Line: &near, Body: "tenant scoping is missing here"},
		{Author: "bob", CommentType: "review_comment", Path: &path, Line: &far, Body: "unrelated nit at the top"},
		{Author: "bob", CommentType: "review_comment", Path: strPtr("src/other.ts"), Line: &other, Body: "another file"},
	}
	steps := []appstore.TourStep{
		{File: path, Anchor: "func handleReferrals(", What: "scoped the query"},
		{File: path, Anchor: "func handleReferrals(", What: "already annotated", Review: "@alice — keep"},
	}
	attachReviewRemarks(repo, steps, comments)
	assert.Equal(t, "@bob src/x.ts:110 — tenant scoping is missing here", steps[0].Review, "nearest remark on the same file")
	assert.Equal(t, "@alice — keep", steps[1].Review, "model-provided remark untouched")

	assert.Contains(t, formatReviewRemarks(comments[:1]), "- @bob src/x.ts:110 — tenant scoping is missing here")
}

func strPtr(s string) *string { return &s }
