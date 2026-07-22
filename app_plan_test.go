package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWritePlanTempFile(t *testing.T) {
	path, err := writePlanTempFile("plan-123", "# Plan\nbody")
	require.NoError(t, err)
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "# Plan\nbody", string(b))
	assert.Contains(t, path, "cc-plan-plan-123")
}
