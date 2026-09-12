// Fallback tour generation: when an agent leaves no `.vibecodeway/tour.json`
// contract behind, the app builds one itself from the repo's working-tree
// diff and a tail of the terminal transcript. See
// docs/superpowers/specs/2026-09-11-code-tour-design.md §3.
package tour

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/artschekoff/vibecodeway/internal/openai"
)

// AIClient is the ChatCompletion-shaped subset of internal/openai's Client
// that Generate needs, extracted as an interface (rather than depending on
// *openai.Client directly) so tests can fake the LLM without a real API key
// or a store-backed settings row. internal/openai.Client already has exactly
// this method signature, so it satisfies AIClient with no wrapping needed.
type AIClient interface {
	ChatCompletion(ctx context.Context, messages []openai.Message) (*openai.Response, error)
}

// Prompt caps, per spec §3: the diff and transcript tail are each capped
// before being sent to the LLM so a huge working tree or a chatty terminal
// session can't blow the prompt budget.
const (
	diffPromptCap       = 24 * 1024 // ~24KB of diff text
	transcriptPromptCap = 16 * 1024 // ~16KB of transcript tail
	untrackedFileCap    = 4 * 1024  // per-untracked-file cap inside CollectDiff
)

// generateSystemPrompt demands strict JSON matching the tour.json schema
// (spec §1 TourStep / §2 file contract).
const generateSystemPrompt = `You are generating a guided "code tour" from a git diff and a terminal transcript of the coding session that produced it.

Respond with STRICT JSON only — no prose, no markdown code fences, no commentary before or after — matching exactly this schema:

{
  "version": 1,
  "title": "short title for the change",
  "steps": [
    {
      "file": "repo-relative path",
      "anchor": "a unique verbatim snippet copied from the post-change file content",
      "what": "what changed at this spot",
      "why": "why it changed"
    }
  ]
}

Rules:
- Produce between 3 and 10 steps, ordered to tell the story of the change (e.g. entry point before detail, setup before consequence) — not just the order files happen to appear in the diff.
- "anchor" MUST be copied verbatim, character for character, from the file's current (post-change) content, and MUST be unique within that file — the UI finds the step by searching the live file for this exact text. Never invent an anchor or normalize whitespace.
- Every step needs a non-empty "file", "anchor", and "what". "why" may be empty, but fill it in from the transcript's stated reasoning whenever it's there.
- Only reference files and code that actually appear in the diff below — never invent files.
- Output ONLY the JSON object described above. Nothing else.`

// Generate produces a fallback tour from repoPath's working-tree diff and a
// tail of the terminal transcript (spec §3). It makes at most two LLM calls
// via ai.ChatCompletion — no tools, so ai_usage accounting applies
// automatically at the dispatch level — retrying once if the first response
// is not valid tour JSON, with the validation error appended to the retry
// prompt. If both attempts are invalid, the returned error wraps both
// failures (and ErrInvalid).
func Generate(ctx context.Context, ai AIClient, repoPath, transcriptTail string) (*File, error) {
	diff, err := CollectDiff(repoPath)
	if err != nil {
		return nil, fmt.Errorf("collect diff: %w", err)
	}

	messages := []openai.Message{
		{Role: "system", Content: generateSystemPrompt},
		{Role: "user", Content: buildUserPrompt(diff, transcriptTail)},
	}

	file, firstErr := callAndParse(ctx, ai, messages)
	if firstErr == nil {
		return file, nil
	}

	retryMessages := append(append([]openai.Message{}, messages...), openai.Message{
		Role: "user",
		Content: fmt.Sprintf(
			"Your previous response was invalid: %s\n\nRespond again with ONLY the corrected strict JSON matching the schema — no prose, no code fences.",
			firstErr,
		),
	})
	file, secondErr := callAndParse(ctx, ai, retryMessages)
	if secondErr == nil {
		return file, nil
	}

	return nil, fmt.Errorf("generate tour: both attempts invalid: first: %v; retry: %w", firstErr, secondErr)
}

// callAndParse makes one LLM call and parses+validates the response through
// Task 2's existing tour.json validation, so a generated tour is held to the
// same schema rules as an agent-authored one.
func callAndParse(ctx context.Context, ai AIClient, messages []openai.Message) (*File, error) {
	resp, err := ai.ChatCompletion(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}

	var f File
	if err := json.Unmarshal([]byte(extractJSON(resp.Content)), &f); err != nil {
		return nil, fmt.Errorf("%w: parse response json: %v", ErrInvalid, err)
	}
	if err := validate(&f); err != nil {
		return nil, err
	}
	return &f, nil
}

// extractJSON strips a ```json / ``` fence if the model wrapped its output
// in one despite instructions not to; otherwise returns s trimmed.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// buildUserPrompt assembles the diff and transcript tail into the user
// message, capping each and noting when truncation happened. The diff keeps
// its head (earlier files in the diff matter most and stay intact); the
// transcript keeps its tail (the most recent reasoning is the most relevant,
// and it is already "the tail" of a larger transcript).
func buildUserPrompt(diff, transcriptTail string) string {
	diffText, diffTruncated := truncateWithNote(diff, diffPromptCap, false)
	transcriptText, transcriptTruncated := truncateWithNote(transcriptTail, transcriptPromptCap, true)

	var b strings.Builder
	b.WriteString("## Working-tree diff\n")
	if diffTruncated {
		fmt.Fprintf(&b, "(truncated to the first %d bytes)\n", diffPromptCap)
	}
	b.WriteString("```diff\n")
	b.WriteString(diffText)
	b.WriteString("\n```\n\n")

	b.WriteString("## Terminal transcript tail\n")
	if transcriptTruncated {
		fmt.Fprintf(&b, "(truncated to the last %d bytes)\n", transcriptPromptCap)
	}
	b.WriteString("```\n")
	b.WriteString(transcriptText)
	b.WriteString("\n```\n")

	return b.String()
}

// truncateWithNote caps s to at most limit bytes, keeping either the head
// (keepTail=false) or the tail (keepTail=true), and reports whether
// truncation happened.
func truncateWithNote(s string, limit int, keepTail bool) (out string, truncated bool) {
	if len(s) <= limit {
		return s, false
	}
	if keepTail {
		return s[len(s)-limit:], true
	}
	return s[:limit], true
}

// CollectDiff returns the unified diff text of repoPath's working tree:
// unstaged changes (`git diff`), staged changes (`git diff --cached`), and
// the content of untracked files (each capped). This is a standalone
// collector rather than a reuse of internal/vcs's UI-shaped FileChange
// structures (used by ChangedFilesPanel / GetContextChanges) — generation
// only needs plain diff text, not per-file status metadata.
func CollectDiff(repoPath string) (string, error) {
	var b strings.Builder

	unstaged, err := runGit(repoPath, "diff")
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	if strings.TrimSpace(unstaged) != "" {
		b.WriteString("=== unstaged changes (git diff) ===\n")
		b.WriteString(unstaged)
		b.WriteString("\n")
	}

	staged, err := runGit(repoPath, "diff", "--cached")
	if err != nil {
		return "", fmt.Errorf("git diff --cached: %w", err)
	}
	if strings.TrimSpace(staged) != "" {
		b.WriteString("=== staged changes (git diff --cached) ===\n")
		b.WriteString(staged)
		b.WriteString("\n")
	}

	untracked, err := untrackedFiles(repoPath)
	if err != nil {
		return "", fmt.Errorf("list untracked files: %w", err)
	}
	for _, path := range untracked {
		content, rerr := os.ReadFile(filepath.Join(repoPath, path))
		if rerr != nil {
			continue // best-effort: skip unreadable files (binary/permissions/race)
		}
		capped, truncated := truncateWithNote(string(content), untrackedFileCap, false)
		fmt.Fprintf(&b, "=== untracked file: %s ===\n", path)
		b.WriteString(capped)
		if truncated {
			b.WriteString("\n... [truncated]")
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

// untrackedFiles lists repo-relative paths of untracked files in repoPath,
// expanding untracked directories into individual files (matches the
// --untracked-files=all convention internal/vcs's RepoDiff already uses).
func untrackedFiles(repoPath string) ([]string, error) {
	out, err := runGit(repoPath, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if !strings.HasPrefix(line, "?? ") {
			continue
		}
		files = append(files, strings.TrimSpace(strings.TrimPrefix(line, "??")))
	}
	sort.Strings(files)
	return files, nil
}

// runGit runs `git -C repoPath <args...>` and returns stdout, wrapping
// stderr into the error for diagnosability.
func runGit(repoPath string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", repoPath}, args...)...).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, exitErr.Stderr)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}
