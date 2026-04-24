package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextTextsTableExists(t *testing.T) {
	s := openTestStore(t)
	assertTableExists(t, s, "context_texts")
}

func TestUpsertContextText_create(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	ct, err := s.UpsertContextText(ctx, "", c.ID, "My Doc", "Hello world content")
	require.NoError(t, err)
	assert.NotEmpty(t, ct.ID)
	assert.Equal(t, c.ID, ct.ContextID)
	assert.Equal(t, "My Doc", ct.Name)
	assert.Equal(t, "Hello world content", ct.Content)
	assert.NotEmpty(t, ct.CreatedAt)
	assert.NotEmpty(t, ct.UpdatedAt)
}

func TestUpsertContextText_update(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	ct, err := s.UpsertContextText(ctx, "", c.ID, "Original", "original content")
	require.NoError(t, err)

	updated, err := s.UpsertContextText(ctx, ct.ID, c.ID, "Updated Name", "updated content")
	require.NoError(t, err)
	assert.Equal(t, ct.ID, updated.ID)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.Equal(t, "updated content", updated.Content)
}

func TestListContextTexts_empty(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	texts, err := s.ListContextTexts(ctx, c.ID, false)
	require.NoError(t, err)
	assert.Empty(t, texts)
}

func TestListContextTexts_returnsAll(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	_, err = s.UpsertContextText(ctx, "", c.ID, "Doc 1", "content one")
	require.NoError(t, err)
	_, err = s.UpsertContextText(ctx, "", c.ID, "Doc 2", "content two")
	require.NoError(t, err)

	texts, err := s.ListContextTexts(ctx, c.ID, false)
	require.NoError(t, err)
	assert.Len(t, texts, 2)
}

func TestListContextTexts_isolatedByContext(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c1, err := s.CreateContext(ctx, "ctx-1")
	require.NoError(t, err)
	c2, err := s.CreateContext(ctx, "ctx-2")
	require.NoError(t, err)

	_, err = s.UpsertContextText(ctx, "", c1.ID, "Doc A", "content a")
	require.NoError(t, err)
	_, err = s.UpsertContextText(ctx, "", c2.ID, "Doc B", "content b")
	require.NoError(t, err)

	texts1, err := s.ListContextTexts(ctx, c1.ID, false)
	require.NoError(t, err)
	assert.Len(t, texts1, 1)
	assert.Equal(t, "Doc A", texts1[0].Name)

	texts2, err := s.ListContextTexts(ctx, c2.ID, false)
	require.NoError(t, err)
	assert.Len(t, texts2, 1)
	assert.Equal(t, "Doc B", texts2[0].Name)
}

func TestDeleteContextText(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	ct, err := s.UpsertContextText(ctx, "", c.ID, "To Delete", "some content")
	require.NoError(t, err)

	err = s.DeleteContextText(ctx, ct.ID)
	require.NoError(t, err)

	texts, err := s.ListContextTexts(ctx, c.ID, false)
	require.NoError(t, err)
	assert.Empty(t, texts)
}

func TestDeleteContextText_notFound(t *testing.T) {
	s := openTestStore(t)
	err := s.DeleteContextText(context.Background(), "non-existent-id")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestContextTextsDeletedWithContext(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	_, err = s.UpsertContextText(ctx, "", c.ID, "Doc", "content")
	require.NoError(t, err)

	err = s.DeleteContext(ctx, c.ID)
	require.NoError(t, err)

	// context_texts should be cascade-deleted
	texts, err := s.ListContextTexts(ctx, c.ID, false)
	require.NoError(t, err)
	assert.Empty(t, texts)
}

// --- IsActive toggle ---

func TestContextTextActiveByDefault(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)
	_, err = s.UpsertContextText(ctx, "", c.ID, "Doc", "content")
	require.NoError(t, err)

	texts, err := s.ListContextTexts(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, texts, 1)
	assert.True(t, texts[0].IsActive, "new text should be active by default")
}

func TestToggleContextTextActive(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)
	ct, err := s.UpsertContextText(ctx, "", c.ID, "Doc", "content")
	require.NoError(t, err)

	toggled, err := s.ToggleContextTextActive(ctx, ct.ID)
	require.NoError(t, err)
	assert.False(t, toggled.IsActive)

	toggled, err = s.ToggleContextTextActive(ctx, ct.ID)
	require.NoError(t, err)
	assert.True(t, toggled.IsActive)
}

func TestToggleContextTextActive_NotFound(t *testing.T) {
	s := openTestStore(t)
	_, err := s.ToggleContextTextActive(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestUpsertContextText_largeContent(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	// Simulate large text content (e.g., 100KB)
	largeContent := make([]byte, 100*1024)
	for i := range largeContent {
		largeContent[i] = 'a' + byte(i%26)
	}

	ct, err := s.UpsertContextText(ctx, "", c.ID, "Large Doc", string(largeContent))
	require.NoError(t, err)
	assert.Equal(t, string(largeContent), ct.Content)
}

// --- Uploaded files ---

func TestUploadedFilesTableExists(t *testing.T) {
	s := openTestStore(t)
	assertTableExists(t, s, "context_uploaded_files")
}

func TestInsertUploadedFile(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	f, err := s.InsertUploadedFile(ctx, c.ID, "report.pdf", "pdf", "extracted text here", []byte("raw-pdf-bytes"), 42)
	require.NoError(t, err)
	assert.NotEmpty(t, f.ID)
	assert.Equal(t, c.ID, f.ContextID)
	assert.Equal(t, "report.pdf", f.OriginalName)
	assert.Equal(t, "pdf", f.FileType)
	assert.Equal(t, "extracted text here", f.ExtractedText)
	assert.Equal(t, int64(42), f.FileSize)
	assert.True(t, f.IsActive)
	assert.NotEmpty(t, f.CreatedAt)
}

func TestListUploadedFiles(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	_, err = s.InsertUploadedFile(ctx, c.ID, "a.pdf", "pdf", "text a", []byte{}, 10)
	require.NoError(t, err)
	_, err = s.InsertUploadedFile(ctx, c.ID, "b.docx", "docx", "text b", []byte{}, 20)
	require.NoError(t, err)

	files, err := s.ListUploadedFiles(ctx, c.ID, false)
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestListUploadedFiles_activeOnly(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	f1, err := s.InsertUploadedFile(ctx, c.ID, "a.pdf", "pdf", "text a", []byte{}, 10)
	require.NoError(t, err)
	_, err = s.InsertUploadedFile(ctx, c.ID, "b.docx", "docx", "text b", []byte{}, 20)
	require.NoError(t, err)

	// Deactivate first file
	_, err = s.ToggleUploadedFileActive(ctx, f1.ID)
	require.NoError(t, err)

	files, err := s.ListUploadedFiles(ctx, c.ID, true)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, "b.docx", files[0].OriginalName)
}

func TestDeleteUploadedFile(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	f, err := s.InsertUploadedFile(ctx, c.ID, "report.pdf", "pdf", "text", []byte{}, 10)
	require.NoError(t, err)

	err = s.DeleteUploadedFile(ctx, f.ID)
	require.NoError(t, err)

	files, err := s.ListUploadedFiles(ctx, c.ID, false)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestDeleteUploadedFile_notFound(t *testing.T) {
	s := openTestStore(t)
	err := s.DeleteUploadedFile(context.Background(), "non-existent-id")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetUploadedFileContent(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	content := []byte("binary-content-here")
	f, err := s.InsertUploadedFile(ctx, c.ID, "doc.pdf", "pdf", "extracted", content, int64(len(content)))
	require.NoError(t, err)

	got, err := s.GetUploadedFileContent(ctx, f.ID)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestUploadedFilesDeletedWithContext(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	_, err = s.InsertUploadedFile(ctx, c.ID, "report.pdf", "pdf", "text", []byte{}, 10)
	require.NoError(t, err)

	err = s.DeleteContext(ctx, c.ID)
	require.NoError(t, err)

	files, err := s.ListUploadedFiles(ctx, c.ID, false)
	require.NoError(t, err)
	assert.Empty(t, files)
}
