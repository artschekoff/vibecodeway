package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextFilesTableExists(t *testing.T) {
	s := openTestStore(t)
	assertTableExists(t, s, "context_files")
}

func TestUpsertContextFile_create(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	f, err := s.UpsertContextFile(ctx, "", c.ID, "My Doc", "Hello world content")
	require.NoError(t, err)
	assert.NotEmpty(t, f.ID)
	assert.Equal(t, c.ID, f.ContextID)
	assert.Equal(t, "My Doc", f.Name)
	assert.Equal(t, "Hello world content", f.Content)
	assert.NotEmpty(t, f.CreatedAt)
	assert.NotEmpty(t, f.UpdatedAt)
}

func TestUpsertContextFile_update(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	f, err := s.UpsertContextFile(ctx, "", c.ID, "Original", "original content")
	require.NoError(t, err)

	updated, err := s.UpsertContextFile(ctx, f.ID, c.ID, "Updated Name", "updated content")
	require.NoError(t, err)
	assert.Equal(t, f.ID, updated.ID)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.Equal(t, "updated content", updated.Content)
}

func TestListContextFiles_empty(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	files, err := s.ListContextFiles(ctx, c.ID)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestListContextFiles_returnsAll(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	_, err = s.UpsertContextFile(ctx, "", c.ID, "Doc 1", "content one")
	require.NoError(t, err)
	_, err = s.UpsertContextFile(ctx, "", c.ID, "Doc 2", "content two")
	require.NoError(t, err)

	files, err := s.ListContextFiles(ctx, c.ID)
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestListContextFiles_isolatedByContext(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c1, err := s.CreateContext(ctx, "ctx-1")
	require.NoError(t, err)
	c2, err := s.CreateContext(ctx, "ctx-2")
	require.NoError(t, err)

	_, err = s.UpsertContextFile(ctx, "", c1.ID, "Doc A", "content a")
	require.NoError(t, err)
	_, err = s.UpsertContextFile(ctx, "", c2.ID, "Doc B", "content b")
	require.NoError(t, err)

	files1, err := s.ListContextFiles(ctx, c1.ID)
	require.NoError(t, err)
	assert.Len(t, files1, 1)
	assert.Equal(t, "Doc A", files1[0].Name)

	files2, err := s.ListContextFiles(ctx, c2.ID)
	require.NoError(t, err)
	assert.Len(t, files2, 1)
	assert.Equal(t, "Doc B", files2[0].Name)
}

func TestDeleteContextFile(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	f, err := s.UpsertContextFile(ctx, "", c.ID, "To Delete", "some content")
	require.NoError(t, err)

	err = s.DeleteContextFile(ctx, f.ID)
	require.NoError(t, err)

	files, err := s.ListContextFiles(ctx, c.ID)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestDeleteContextFile_notFound(t *testing.T) {
	s := openTestStore(t)
	err := s.DeleteContextFile(context.Background(), "non-existent-id")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestContextFilesDeletedWithContext(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	_, err = s.UpsertContextFile(ctx, "", c.ID, "Doc", "content")
	require.NoError(t, err)

	err = s.DeleteContext(ctx, c.ID)
	require.NoError(t, err)

	// context_files should be cascade-deleted
	files, err := s.ListContextFiles(ctx, c.ID)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestUpsertContextFile_largeContent(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	// Simulate large text content (e.g., 100KB)
	largeContent := make([]byte, 100*1024)
	for i := range largeContent {
		largeContent[i] = 'a' + byte(i%26)
	}

	f, err := s.UpsertContextFile(ctx, "", c.ID, "Large Doc", string(largeContent))
	require.NoError(t, err)
	assert.Equal(t, string(largeContent), f.Content)
}
