package contextlog_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachFile_and_ListLogs_populatesFiles(t *testing.T) {
	s, svc := openTestSvc(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "test-ctx")
	require.NoError(t, err)

	log, err := svc.CreateManualLog(ctx, c.ID, "notes")
	require.NoError(t, err)

	f, err := s.InsertUploadedFile(ctx, c.ID, "spec.pdf", "pdf", "extracted text", []byte{}, 10)
	require.NoError(t, err)

	err = svc.AttachFile(ctx, log.ID, f.ID)
	require.NoError(t, err)

	logs, err := svc.ListLogs(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Len(t, logs[0].Files, 1)
	assert.Equal(t, f.ID, logs[0].Files[0].FileID)
	assert.Equal(t, "spec.pdf", logs[0].Files[0].OriginalName)
	assert.Equal(t, "pdf", logs[0].Files[0].FileType)
	assert.Equal(t, "extracted text", logs[0].Files[0].ExtractedText)
	assert.NotEmpty(t, logs[0].Files[0].CreatedAt)
}

func TestAttachFile_idempotent(t *testing.T) {
	s, svc := openTestSvc(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "test-ctx")
	require.NoError(t, err)

	log, err := svc.CreateManualLog(ctx, c.ID, "notes")
	require.NoError(t, err)

	f, err := s.InsertUploadedFile(ctx, c.ID, "doc.pdf", "pdf", "content", []byte{}, 5)
	require.NoError(t, err)

	require.NoError(t, svc.AttachFile(ctx, log.ID, f.ID))
	require.NoError(t, svc.AttachFile(ctx, log.ID, f.ID), "second attach should be idempotent")

	logs, err := svc.ListLogs(ctx, c.ID)
	require.NoError(t, err)
	assert.Len(t, logs[0].Files, 1, "should not duplicate")
}

func TestDetachFile_removesLink(t *testing.T) {
	s, svc := openTestSvc(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "test-ctx")
	require.NoError(t, err)

	log, err := svc.CreateManualLog(ctx, c.ID, "notes")
	require.NoError(t, err)

	f, err := s.InsertUploadedFile(ctx, c.ID, "doc.pdf", "pdf", "content", []byte{}, 5)
	require.NoError(t, err)

	require.NoError(t, svc.AttachFile(ctx, log.ID, f.ID))
	require.NoError(t, svc.DetachFile(ctx, log.ID, f.ID))

	logs, err := svc.ListLogs(ctx, c.ID)
	require.NoError(t, err)
	assert.Empty(t, logs[0].Files)
}

func TestAttachFile_deletingFileCascades(t *testing.T) {
	s, svc := openTestSvc(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "test-ctx")
	require.NoError(t, err)

	log, err := svc.CreateManualLog(ctx, c.ID, "notes")
	require.NoError(t, err)

	f, err := s.InsertUploadedFile(ctx, c.ID, "doc.pdf", "pdf", "content", []byte{}, 5)
	require.NoError(t, err)

	require.NoError(t, svc.AttachFile(ctx, log.ID, f.ID))
	require.NoError(t, s.DeleteUploadedFile(ctx, f.ID))

	logs, err := svc.ListLogs(ctx, c.ID)
	require.NoError(t, err)
	assert.Empty(t, logs[0].Files, "cascade delete should remove log file link")
}
