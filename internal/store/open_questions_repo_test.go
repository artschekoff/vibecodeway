package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenQuestionsRepo_CRUD(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "oq-ctx", org.ID)
	require.NoError(t, err)

	q, err := s.CreateOpenQuestion(ctx, c.ID, "What is the deadline?", "ai")
	require.NoError(t, err)
	assert.Equal(t, "open", q.Status)

	qs, err := s.ListOpenQuestions(ctx, c.ID)
	require.NoError(t, err)
	assert.Len(t, qs, 1)

	require.NoError(t, s.AnswerOpenQuestion(ctx, q.ID, "End of Q2"))
	qs, _ = s.ListOpenQuestions(ctx, c.ID)
	assert.Equal(t, "answered", qs[0].Status)
	assert.Equal(t, "End of Q2", *qs[0].Answer)

	q2, _ := s.CreateOpenQuestion(ctx, c.ID, "Who is the owner?", "ai")
	require.NoError(t, s.DismissOpenQuestion(ctx, q2.ID))

	open, _ := s.ListOpenOpenQuestions(ctx, c.ID)
	assert.Len(t, open, 0)
}
