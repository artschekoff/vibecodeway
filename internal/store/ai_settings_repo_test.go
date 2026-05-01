package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAISettings_DefaultProvider(t *testing.T) {
	s := openTestStore(t)
	settings, err := s.GetAISettings(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "openai", settings.Provider)
	assert.False(t, settings.HasOpenAIKey)
	assert.False(t, settings.HasAnthropicKey)
}

func TestSetAISettings_AnthropicProvider(t *testing.T) {
	s := openTestStore(t)
	err := s.SetAISettings(context.Background(), SetAISettingsStoreOpts{
		Provider:     "anthropic",
		AnthropicKey: "sk-ant-test",
		Model:        "claude-sonnet-4-6",
		Passphrase:   "testpassphrase",
	})
	require.NoError(t, err)

	settings, err := s.GetAISettings(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "anthropic", settings.Provider)
	assert.True(t, settings.HasAnthropicKey)
	assert.False(t, settings.HasOpenAIKey)
	assert.Equal(t, "claude-sonnet-4-6", settings.Model)

	key, err := s.GetAIAnthropicKey(context.Background(), "testpassphrase")
	require.NoError(t, err)
	assert.Equal(t, "sk-ant-test", key)
}
