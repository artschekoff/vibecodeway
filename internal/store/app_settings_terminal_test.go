package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppSettings_CodingAgent(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	t.Run("default CodingAgentApp is cursor", func(t *testing.T) {
		settings, err := s.GetAppSettings(ctx)
		require.NoError(t, err)
		assert.Equal(t, "cursor", settings.CodingAgentApp)
	})

	t.Run("SetAppSettings persists CodingAgentApp", func(t *testing.T) {
		err := s.SetAppSettings(ctx, AppSettings{
			ContextRefetchEnabled:     false,
			ContextRefetchIntervalMin: 30,
			ReviewsRefetchEnabled:     false,
			ReviewsRefetchIntervalMin: 30,
			TerminalAppPath:           "kitty",
			CodingAgentApp:            "code",
		})
		require.NoError(t, err)

		got, err := s.GetAppSettings(ctx)
		require.NoError(t, err)
		assert.Equal(t, "code", got.CodingAgentApp)
	})
}

func TestAppSettings_Terminal(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	t.Run("default TerminalAppPath is empty", func(t *testing.T) {
		settings, err := s.GetAppSettings(ctx)
		require.NoError(t, err)
		assert.Equal(t, "", settings.TerminalAppPath)
	})

	t.Run("SetAppSettings persists TerminalAppPath", func(t *testing.T) {
		err := s.SetAppSettings(ctx, AppSettings{
			ContextRefetchEnabled:     false,
			ContextRefetchIntervalMin: 30,
			ReviewsRefetchEnabled:     false,
			ReviewsRefetchIntervalMin: 30,
			TerminalAppPath:           "kitty",
			CodingAgentApp:            "cursor",
		})
		require.NoError(t, err)

		got, err := s.GetAppSettings(ctx)
		require.NoError(t, err)
		assert.Equal(t, "kitty", got.TerminalAppPath)
	})
}
