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

func TestSetAISettings_PreservesKeyOnModelUpdate(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// Insert with an OpenAI key
	err := s.SetAISettings(ctx, SetAISettingsStoreOpts{
		Provider:   "openai",
		OpenAIKey:  "sk-original",
		Model:      "gpt-4o",
		Passphrase: "testpassphrase",
	})
	require.NoError(t, err)

	// Update only the model — key must be preserved
	err = s.SetAISettings(ctx, SetAISettingsStoreOpts{
		Model: "gpt-4o-mini",
	})
	require.NoError(t, err)

	settings, err := s.GetAISettings(ctx)
	require.NoError(t, err)
	assert.True(t, settings.HasOpenAIKey, "key must still be set after model-only update")
	assert.Equal(t, "gpt-4o-mini", settings.Model)

	key, err := s.GetAIOpenAIKey(ctx, "testpassphrase")
	require.NoError(t, err)
	assert.Equal(t, "sk-original", key, "key value must not have changed")
}
