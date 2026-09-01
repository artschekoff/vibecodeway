package main

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The original bytes were stored all along but had no route to the UI: the store
// could read them and nothing exposed them. This binding is that route.
func TestGetUploadedFileContentReturnsTheOriginalBytes(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "PROJ-1", org.ID)
	require.NoError(t, err)

	// A PNG header followed by a NUL — proves the blob survives round-trip
	// unmangled, which a text column would not manage.
	png := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0xFF}
	f, err := s.InsertUploadedFile(ctx, c.ID, "shot.png", "png", "extracted words", png, int64(len(png)))
	require.NoError(t, err)

	a := &App{db: s, ctx: ctx, logger: newTestLogger()}

	got, err := a.GetUploadedFileContent(f.ID)
	require.NoError(t, err)
	decoded, err := base64.StdEncoding.DecodeString(got)
	require.NoError(t, err)
	assert.Equal(t, png, decoded)

	// The extracted text is still there — the original is stored *in addition to*
	// it, not instead of it.
	list, err := s.ListUploadedFiles(ctx, c.ID, true)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "extracted words", list[0].ExtractedText)
}

func TestGetUploadedFileContentUnknownID(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	a := &App{db: s, ctx: context.Background(), logger: newTestLogger()}
	_, err = a.GetUploadedFileContent("nope")
	require.Error(t, err)
}
