package openquestions_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/artschekoff/context-consolidator/internal/openquestions"
	"github.com/artschekoff/context-consolidator/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAI struct{ response string }

func (m *mockAI) ChatCompletion(_ context.Context, _ []openquestions.AIMessage) (*openquestions.AIResponse, error) {
	return &openquestions.AIResponse{Content: m.response}, nil
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestAnalyzeContext_ExtractsQuestions(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	org, _ := s.CreateOrganization(ctx, "test-org")
	c, _ := s.CreateContext(ctx, "ai-ctx", org.ID)

	ai := &mockAI{response: `[
        {"question": "What is the deployment environment?", "answer": null},
        {"question": "Which auth provider?", "answer": "OAuth2 via existing provider"}
    ]`}

	svc := openquestions.NewService(s, ai, nil)
	err := svc.AnalyzeContext(ctx, c.ID, "Build a login page.", nil)
	require.NoError(t, err)

	questions, err := s.ListOpenQuestions(ctx, c.ID)
	require.NoError(t, err)
	assert.Len(t, questions, 2)
	assert.Equal(t, "open", questions[0].Status)
	assert.Equal(t, "answered", questions[1].Status)
	assert.Equal(t, "OAuth2 via existing provider", *questions[1].Answer)
}

func TestAnalyzeContext_NoAI_NoOp(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	org, _ := s.CreateOrganization(ctx, "test-org")
	c, _ := s.CreateContext(ctx, "noai-ctx", org.ID)
	svc := openquestions.NewService(s, nil, nil)
	assert.NoError(t, svc.AnalyzeContext(ctx, c.ID, "content", nil))
}
