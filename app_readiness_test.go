package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveReadinessRejectsUnknownActions(t *testing.T) {
	a := &App{}
	assert.Error(t, a.ResolveReadiness("nope"))
	assert.Error(t, a.ResolveReadiness("bogus:x"))
	assert.NoError(t, a.ResolveReadiness("app-settings:terminal"), "routed by the frontend")
}
