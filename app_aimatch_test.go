package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDedupeMatches(t *testing.T) {
	in := []MatchedBranch{
		{Platform: "github", OwnerRepo: "a/b", Branch: "feat/x"},
		{Platform: "github", OwnerRepo: "a/b", Branch: "feat/x"}, // dup
		{Platform: "github", OwnerRepo: "a/b", Branch: "main"},   // default
	}
	out := dedupeMatches(in)
	require.Len(t, out, 1)
	assert.Equal(t, "feat/x", out[0].Branch)
}
