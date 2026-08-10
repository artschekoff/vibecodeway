package vcs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoneDeletableBranches(t *testing.T) {
	in := []LocalBranch{
		{Name: "main", IsCurrent: true, IsGone: false},
		{Name: "feat-a", IsCurrent: false, IsGone: true},  // deletable
		{Name: "feat-b", IsCurrent: false, IsGone: false}, // not gone
		{Name: "feat-c", IsCurrent: true, IsGone: true},   // gone but current -> skip
		{Name: "feat-d", IsCurrent: false, IsGone: true},  // deletable
	}
	assert.Equal(t, []string{"feat-a", "feat-d"}, GoneDeletableBranches(in))
	assert.Empty(t, GoneDeletableBranches(nil))
}
