package tour

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAIClient is a scripted AIClient: each ChatCompletion call returns the
// next response/error pair in order, and every call's messages are recorded
// so tests can inspect what the retry prompt actually contained.
type fakeAIClient struct {
	responses []string // response content, in call order
	errs      []error  // parallel per-call error (nil = no error for that call)
	calls     [][]openai.Message
}

func (f *fakeAIClient) ChatCompletion(_ context.Context, messages []openai.Message) (*openai.Response, error) {
	f.calls = append(f.calls, messages)
	i := len(f.calls) - 1
	if i < len(f.errs) && f.errs[i] != nil {
		return nil, f.errs[i]
	}
	content := ""
	if i < len(f.responses) {
		content = f.responses[i]
	}
	return &openai.Response{Content: content}, nil
}

// repoWithChanges returns a temp git repo with a genuine uncommitted
// working-tree diff, so Generate doesn't short-circuit on ErrNoChanges.
func repoWithChanges(t *testing.T) string {
	t.Helper()
	repoPath := initGitRepo(t)
	commitAt(t, repoPath, "2026-01-01T00:00:00+00:00") // commits a.txt
	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "a.txt"), []byte("modified for generate test"), 0o644))
	return repoPath
}

const validTourJSON = `{
	"version": 1,
	"title": "Add analytics dashboard",
	"steps": [
		{"file": "src/x.ts", "anchor": "func computeDashboard(", "what": "Added entry point", "why": "keeps aggregation testable"},
		{"file": "src/y.ts", "anchor": "renderChart(", "what": "Wired the chart", "why": "surfaces the numbers"},
		{"file": "src/z.ts", "anchor": "export const Dashboard", "what": "New page", "why": "entry point for the feature"}
	]
}`

func TestGenerate_ValidJSONOnFirstTry(t *testing.T) {
	repoPath := repoWithChanges(t)
	fake := &fakeAIClient{responses: []string{validTourJSON}}

	f, err := Generate(context.Background(), fake, repoPath, "transcript tail here", "")
	require.NoError(t, err)
	require.NotNil(t, f)
	assert.Equal(t, 1, f.Version)
	assert.Equal(t, "Add analytics dashboard", f.Title)
	assert.Len(t, f.Steps, 3)
	assert.Len(t, fake.calls, 1, "only one call needed when the first response is valid")

	// The prompt actually carries the diff/transcript sections.
	userMsg := fake.calls[0][1].Content
	assert.Contains(t, userMsg, "Working-tree diff")
	assert.Contains(t, userMsg, "Terminal transcript tail")
	assert.Contains(t, userMsg, "transcript tail here")
}

func TestGenerate_RetriesOnceOnInvalidJSONThenSucceeds(t *testing.T) {
	repoPath := repoWithChanges(t)
	fake := &fakeAIClient{responses: []string{"not json at all", validTourJSON}}

	f, err := Generate(context.Background(), fake, repoPath, "transcript", "")
	require.NoError(t, err)
	require.NotNil(t, f)
	assert.Equal(t, "Add analytics dashboard", f.Title)

	require.Len(t, fake.calls, 2, "invalid first response must trigger exactly one retry")

	// Attempt 2's prompt must contain attempt 1's error so the model can
	// self-correct.
	secondCallMessages := fake.calls[1]
	lastMsg := secondCallMessages[len(secondCallMessages)-1].Content
	assert.Contains(t, lastMsg, "previous response was invalid")
	assert.Contains(t, lastMsg, "parse response json")
}

func TestGenerate_BothAttemptsInvalid_ReturnsWrappedError(t *testing.T) {
	repoPath := repoWithChanges(t)
	fake := &fakeAIClient{responses: []string{"still not json", "also not json"}}

	f, err := Generate(context.Background(), fake, repoPath, "transcript", "")
	require.Error(t, err)
	assert.Nil(t, f)
	assert.True(t, errors.Is(err, ErrInvalid), "wrapped error must still satisfy errors.Is(err, ErrInvalid)")
	require.Len(t, fake.calls, 2)
}

func TestGenerate_ValidJSONButFailsSchemaValidation_RetriesThenFails(t *testing.T) {
	repoPath := repoWithChanges(t)
	// Well-formed JSON, but missing required fields -> fails validate(), not
	// json.Unmarshal. Both attempts fail this way.
	invalidSchema := `{"version": 1, "title": "x", "steps": []}`
	fake := &fakeAIClient{responses: []string{invalidSchema, invalidSchema}}

	f, err := Generate(context.Background(), fake, repoPath, "transcript", "")
	require.Error(t, err)
	assert.Nil(t, f)
	assert.True(t, errors.Is(err, ErrInvalid))
	require.Len(t, fake.calls, 2)
}

func TestGenerate_LLMCallError_Propagates(t *testing.T) {
	repoPath := repoWithChanges(t)
	fake := &fakeAIClient{errs: []error{errors.New("boom"), errors.New("boom again")}}

	_, err := Generate(context.Background(), fake, repoPath, "transcript", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom again")
}

func TestGenerate_EmptyDiff_ReturnsErrNoChangesWithoutCallingTheLLM(t *testing.T) {
	repoPath := initGitRepo(t)
	commitAt(t, repoPath, "2026-01-01T00:00:00+00:00") // committed, clean tree: no diff

	fake := &fakeAIClient{responses: []string{validTourJSON}}

	f, err := Generate(context.Background(), fake, repoPath, "transcript", "")
	require.Error(t, err)
	assert.Nil(t, f)
	assert.True(t, errors.Is(err, ErrNoChanges), "expected errors.Is(err, ErrNoChanges), got %v", err)
	assert.Empty(t, fake.calls, "no LLM call should happen when there is nothing to tour")
}

func TestExtractJSON_StripsMarkdownFence(t *testing.T) {
	assert.Equal(t, `{"a":1}`, extractJSON("```json\n{\"a\":1}\n```"))
	assert.Equal(t, `{"a":1}`, extractJSON("```\n{\"a\":1}\n```"))
	assert.Equal(t, `{"a":1}`, extractJSON(`{"a":1}`))
}

func TestTruncateWithNote(t *testing.T) {
	s := strings.Repeat("a", 40) + strings.Repeat("b", 40) // 80 bytes

	out, truncated := truncateWithNote(s, 200, false)
	assert.False(t, truncated)
	assert.Equal(t, s, out)

	out, truncated = truncateWithNote(s, 40, false) // keep head
	assert.True(t, truncated)
	assert.Equal(t, strings.Repeat("a", 40), out)

	out, truncated = truncateWithNote(s, 40, true) // keep tail
	assert.True(t, truncated)
	assert.Equal(t, strings.Repeat("b", 40), out)
}

// CollectDiff: a modified tracked file and a new untracked file must both
// appear in the collected diff text.
func TestCollectDiff_ModifiedAndUntrackedBothAppear(t *testing.T) {
	repoPath := initGitRepo(t)
	commitAt(t, repoPath, "2026-01-01T00:00:00+00:00") // commits a.txt with that content

	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "a.txt"), []byte("modified-marker-content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "new_untracked.txt"), []byte("untracked-marker-content"), 0o644))

	diff, err := CollectDiff(repoPath)
	require.NoError(t, err)
	assert.Contains(t, diff, "modified-marker-content")
	assert.Contains(t, diff, "new_untracked.txt")
	assert.Contains(t, diff, "untracked-marker-content")
}

func TestCollectDiff_NoChanges_ReturnsEmptyNoError(t *testing.T) {
	repoPath := initGitRepo(t)
	commitAt(t, repoPath, "2026-01-01T00:00:00+00:00")

	diff, err := CollectDiff(repoPath)
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(diff))
}
