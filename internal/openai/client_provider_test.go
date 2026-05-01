package openai_test

import (
	"testing"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/stretchr/testify/assert"
)

// TestProviderConstantsExist verifies the provider constant strings are defined.
func TestProviderConstantsExist(t *testing.T) {
	assert.Equal(t, "openai", appstore.ProviderOpenAI)
	assert.Equal(t, "anthropic", appstore.ProviderAnthropic)
	assert.Equal(t, "local", appstore.ProviderLocal)
}
