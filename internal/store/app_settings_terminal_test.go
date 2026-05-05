package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		})
		require.NoError(t, err)

		got, err := s.GetAppSettings(ctx)
		require.NoError(t, err)
		assert.Equal(t, "kitty", got.TerminalAppPath)
	})
}
