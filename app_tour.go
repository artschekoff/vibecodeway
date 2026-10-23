package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	openaiclient "github.com/artschekoff/vibecodeway/internal/openai"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/artschekoff/vibecodeway/internal/tour"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"go.uber.org/zap"
)

// tourContractRelPath mirrors internal/tour's package-private contract
// location (spec §2: `<repo>/.vibecodeway/tour.json`). internal/tour does
// not export it (Load/Fresh/Consume own all direct access to the file) but
// StartCodeTour needs the file's mtime to run the freshness check, which
// only tour.Fresh(repoPath, fileModTime) exposes.
const tourContractRelPath = ".vibecodeway/tour.json"

// askTourTranscriptCap bounds how much terminal transcript tail
// AskTourQuestion inlines into its prompt. Deliberately smaller than
// internal/tour.Generate's 16KB cap: AskTourQuestion's prompt already
// carries the step, the file's diff section, and the full step list, so a
// smaller transcript slice keeps a single Q&A call cheap.
const askTourTranscriptCap = 8 * 1024

// ErrCodeTourNoChanges is returned by StartCodeTour when there is no
// `.vibecodeway/tour.json` contract (or it wasn't usable) AND the repo's
// working tree has no changes to build a fallback tour from. Spec §5: "Repo
// with no changes → Start tour disabled with a tooltip". The message is
// stable so the frontend can match on it to render that state instead of a
// generic error toast.
var ErrCodeTourNoChanges = errors.New("code tour: no changes to tour")

// StartCodeTour implements the contract-or-generate flow from spec §2-§3:
//
//  1. If `.vibecodeway/tour.json` exists and validates and is fresh (mtime
//     newer than the repo's last commit), import it as source="agent" and
//     delete the file.
//  2. Otherwise (no file, invalid file, or stale file) fall back to
//     generating a tour from the working-tree diff + terminal transcript
//     tail, stored as source="generated". An invalid file logs a warning
//     before falling through, per spec §2's "Invalid file → warning toast +
//     immediate fallback to generation".
//
// Either path replaces any previous tour for (contextID, repoPath) — see
// appstore.UpsertCodeTour.
func (a *App) StartCodeTour(contextID, repoPath string) (*appstore.CodeTour, error) {
	file, loadErr := tour.Load(repoPath)
	switch {
	case loadErr != nil && errors.Is(loadErr, tour.ErrInvalid):
		if a.logger != nil {
			a.logger.Warn("code tour: contract file invalid, falling back to generation",
				zap.String("repoPath", repoPath), zap.Error(loadErr))
		}
		file = nil
	case loadErr != nil:
		return nil, fmt.Errorf("code tour: %w", loadErr)
	case file != nil:
		fresh, freshErr := a.isTourContractFresh(repoPath)
		if freshErr != nil {
			return nil, fmt.Errorf("code tour: %w", freshErr)
		}
		if !fresh {
			// Stale leftover from a previous task — spec §2 says it "must
			// not resurface"; fall through to generation silently (no
			// warning — this isn't a malformed file, just an old one).
			file = nil
		}
	}

	if file != nil {
		return a.importCodeTourFile(contextID, repoPath, file)
	}
	return a.generateCodeTour(contextID, repoPath)
}

// isTourContractFresh stats the contract file to get its mtime and defers
// the actual freshness rule to tour.Fresh (mtime vs last commit time, spec
// §2). A missing file is treated as "not fresh" — StartCodeTour has already
// established file != nil before calling this, but a race (file deleted
// between Load and Stat) degrades gracefully to the generation fallback
// rather than erroring.
func (a *App) isTourContractFresh(repoPath string) (bool, error) {
	info, err := os.Stat(filepath.Join(repoPath, tourContractRelPath))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat tour contract: %w", err)
	}
	return tour.Fresh(repoPath, info.ModTime())
}

// importCodeTourFile stores file as an agent-authored tour and consumes
// (deletes) the contract file, per spec §2.
func (a *App) importCodeTourFile(contextID, repoPath string, file *tour.File) (*appstore.CodeTour, error) {
	stored := &appstore.CodeTour{
		ContextID: contextID,
		RepoPath:  repoPath,
		Source:    "agent",
		Title:     file.Title,
		Steps:     file.Steps,
		StepIndex: 0,
		Decisions: map[int]string{},
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	attachReviewRemarks(repoPath, stored.Steps, a.reviewRemarks(contextID))
	if err := a.db.UpsertCodeTour(a.ctx, stored); err != nil {
		return nil, fmt.Errorf("code tour: %w", err)
	}
	if err := tour.Consume(repoPath); err != nil {
		return nil, fmt.Errorf("code tour: %w", err)
	}
	return stored, nil
}

// generateCodeTour builds a tour from the working-tree diff + terminal
// transcript tail (spec §3) and stores it as source="generated".
func (a *App) generateCodeTour(contextID, repoPath string) (*appstore.CodeTour, error) {
	transcriptTail := a.ptyMgr.TailForCwd(repoPath)
	remarks := a.reviewRemarks(contextID)

	generated, err := tour.Generate(a.ctx, a.openAI, repoPath, transcriptTail, formatReviewRemarks(remarks))
	if err != nil {
		if errors.Is(err, tour.ErrNoChanges) {
			return nil, ErrCodeTourNoChanges
		}
		return nil, fmt.Errorf("code tour: %w", err)
	}

	stored := &appstore.CodeTour{
		ContextID: contextID,
		RepoPath:  repoPath,
		Source:    "generated",
		Title:     generated.Title,
		Steps:     generated.Steps,
		StepIndex: 0,
		Decisions: map[int]string{},
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	attachReviewRemarks(repoPath, stored.Steps, remarks)
	if err := a.db.UpsertCodeTour(a.ctx, stored); err != nil {
		return nil, fmt.Errorf("code tour: %w", err)
	}
	return stored, nil
}

// reviewRemarks returns the reviewer comments on the context's pull
// request(s): review bodies and inline review comments by people other than
// the PR author. Empty for contexts without a PR, so ordinary tours are
// unaffected.
func (a *App) reviewRemarks(contextID string) []vcsstore.PRComment {
	if a.vcsStore == nil {
		return nil
	}
	prs, err := a.vcsStore.PullRequests.ListPRsForContext(a.ctx, contextID)
	if err != nil {
		return nil
	}
	var out []vcsstore.PRComment
	for _, pr := range prs {
		comments, err := a.vcsStore.PullRequests.ListCommentsForPR(a.ctx, pr.ID)
		if err != nil {
			continue
		}
		for _, c := range comments {
			if c.Author == pr.Author || strings.TrimSpace(c.Body) == "" {
				continue
			}
			if c.CommentType == "review_comment" || c.CommentType == "review" {
				out = append(out, c)
			}
		}
	}
	return out
}

// formatReviewRemarks renders comments for the generation prompt, one per line.
func formatReviewRemarks(comments []vcsstore.PRComment) string {
	var b strings.Builder
	for _, c := range comments {
		b.WriteString("- ")
		b.WriteString(remarkLabel(c))
		b.WriteString(" ")
		b.WriteString(strings.ReplaceAll(strings.TrimSpace(c.Body), "\n", " "))
		b.WriteString("\n")
	}
	return b.String()
}

// remarkLabel is "@author path:line" (+ " (resolved)"), or "@author" for a
// review body without a location.
func remarkLabel(c vcsstore.PRComment) string {
	s := "@" + c.Author
	if c.Path != nil && *c.Path != "" {
		s += " " + *c.Path
		if c.Line != nil {
			s += fmt.Sprintf(":%d", *c.Line)
		}
	}
	if c.IsResolved {
		s += " (resolved)"
	}
	return s + " —"
}

// reviewRemarkWindow is how far (in lines) an inline comment may sit from a
// step's anchor and still count as the remark that step addresses.
const reviewRemarkWindow = 40

// attachReviewRemarks fills Review for steps the model (or the agent) left
// blank, from inline comments on the step's file nearest to its anchor —
// so the reviewer's words always show next to the change that answers them.
func attachReviewRemarks(repoPath string, steps []appstore.TourStep, comments []vcsstore.PRComment) {
	if len(comments) == 0 {
		return
	}
	for i := range steps {
		if steps[i].Review != "" {
			continue
		}
		anchorLine := anchorLineOf(repoPath, steps[i].File, steps[i].Anchor)
		var picked []string
		for _, c := range comments {
			if c.Path == nil || *c.Path != steps[i].File {
				continue
			}
			if anchorLine > 0 && c.Line != nil && abs(*c.Line-anchorLine) > reviewRemarkWindow {
				continue
			}
			picked = append(picked, remarkLabel(c)+" "+strings.TrimSpace(c.Body))
		}
		steps[i].Review = strings.Join(picked, "\n\n")
	}
}

// anchorLineOf returns the 1-based line where anchor first occurs in the
// file, or 0 when the file or anchor cannot be found.
func anchorLineOf(repoPath, file, anchor string) int {
	b, err := os.ReadFile(filepath.Join(repoPath, file))
	if err != nil {
		return 0
	}
	idx := strings.Index(string(b), anchor)
	if idx < 0 {
		return 0
	}
	return strings.Count(string(b[:idx]), "\n") + 1
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// GetCodeTour returns the active tour for (contextID, repoPath), or nil if
// none exists (thin passthrough to appstore.Store.GetCodeTour).
func (a *App) GetCodeTour(contextID, repoPath string) (*appstore.CodeTour, error) {
	t, err := a.db.GetCodeTour(a.ctx, contextID, repoPath)
	if err != nil {
		return nil, fmt.Errorf("code tour: %w", err)
	}
	return t, nil
}

// SetCodeTourStep updates the current step index for tour id.
func (a *App) SetCodeTourStep(id string, stepIndex int) error {
	if err := a.db.SetCodeTourStep(a.ctx, id, stepIndex); err != nil {
		return fmt.Errorf("code tour: %w", err)
	}
	return nil
}

// SetCodeTourDecision records the decision (staged/discarded/edited/skipped)
// for step stepIdx of tour id.
func (a *App) SetCodeTourDecision(id string, stepIdx int, decision string) error {
	if err := a.db.SetCodeTourDecision(a.ctx, id, stepIdx, decision); err != nil {
		return fmt.Errorf("code tour: %w", err)
	}
	return nil
}

// askTourSystemPrompt keeps AskTourQuestion's answers short and scoped to
// explaining the tour step at hand, per spec §4 ("Q&A: answered inline by
// the app's own LLM").
const askTourSystemPrompt = `You are explaining a code change tour to the developer reviewing it. You are given one step of the tour (a file, an anchor snippet, what changed, and why), that file's current diff, a summary of every step in the tour, and a tail of the terminal transcript from the session that produced the change. Answer the developer's question concisely and precisely, grounded in the material provided. Answer in the same language the developer's question is written in.`

// AskTourQuestion answers a free-form question about tour step stepIdx of
// tour tourID (spec §4). Context assembled for the LLM call: the current
// step's file/what/why/anchor, that file's diff section (naive filter of
// tour.CollectDiff's output by file path — see filterDiffForFile), a
// summary of every step, and a capped tail of the repo's terminal
// transcript. One ChatCompletion call, no tools.
func (a *App) AskTourQuestion(tourID, contextID, repoPath string, stepIdx int, question string) (string, error) {
	t, err := a.db.GetCodeTour(a.ctx, contextID, repoPath)
	if err != nil {
		return "", fmt.Errorf("code tour: %w", err)
	}
	if t == nil {
		return "", fmt.Errorf("code tour: no active tour for context %q repo %q", contextID, repoPath)
	}
	if t.ID != tourID {
		return "", fmt.Errorf("code tour: tour id mismatch (active tour is %q, asked about %q)", t.ID, tourID)
	}
	if stepIdx < 0 || stepIdx >= len(t.Steps) {
		return "", fmt.Errorf("code tour: step index %d out of range (tour has %d steps)", stepIdx, len(t.Steps))
	}
	step := t.Steps[stepIdx]

	diff, err := tour.CollectDiff(repoPath)
	if err != nil {
		return "", fmt.Errorf("code tour: %w", err)
	}
	fileDiff := filterDiffForFile(diff, step.File)

	transcriptTail := a.ptyMgr.TailForCwd(repoPath)
	if len(transcriptTail) > askTourTranscriptCap {
		transcriptTail = transcriptTail[len(transcriptTail)-askTourTranscriptCap:]
	}

	userContent := buildAskTourUserPrompt(t, stepIdx, step, fileDiff, transcriptTail, question)

	resp, err := a.openAI.ChatCompletion(a.ctx, []openaiclient.Message{
		{Role: "system", Content: askTourSystemPrompt},
		{Role: "user", Content: userContent},
	})
	if err != nil {
		return "", fmt.Errorf("code tour: %w", err)
	}
	return resp.Content, nil
}

// buildAskTourUserPrompt assembles the current step, that file's diff
// section, a summary of every tour step, the transcript tail, and the
// question into one user message.
func buildAskTourUserPrompt(t *appstore.CodeTour, stepIdx int, step appstore.TourStep, fileDiff, transcriptTail, question string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "## Current step (%d/%d)\n", stepIdx+1, len(t.Steps))
	fmt.Fprintf(&b, "File: %s\n", step.File)
	fmt.Fprintf(&b, "Anchor: %s\n", step.Anchor)
	fmt.Fprintf(&b, "What: %s\n", step.What)
	fmt.Fprintf(&b, "Why: %s\n\n", step.Why)

	b.WriteString("## Diff for this step's file\n```diff\n")
	if strings.TrimSpace(fileDiff) == "" {
		b.WriteString("(no diff section found for this file)\n")
	} else {
		b.WriteString(fileDiff)
	}
	b.WriteString("\n```\n\n")

	b.WriteString("## All tour steps\n")
	for i, s := range t.Steps {
		marker := ""
		if i == stepIdx {
			marker = " (current)"
		}
		fmt.Fprintf(&b, "%d. %s%s — %s\n", i+1, s.File, marker, s.What)
	}
	b.WriteString("\n")

	b.WriteString("## Terminal transcript tail\n```\n")
	if strings.TrimSpace(transcriptTail) == "" {
		b.WriteString("(no transcript available)\n")
	} else {
		b.WriteString(transcriptTail)
	}
	b.WriteString("\n```\n\n")

	b.WriteString("## Question\n")
	b.WriteString(question)

	return b.String()
}

// filterDiffForFile naively extracts the section(s) of diff (the format
// produced by tour.CollectDiff — "=== unstaged changes ===" / "=== staged
// changes ===" blocks of raw `git diff` output, plus "=== untracked file:
// <path> ===" blocks) that mention path. It tracks the "current file" by
// matching `diff --git a/<path> b/<path>` headers and "=== untracked file:
// <path> ===" markers, capturing lines until the next such marker or
// section boundary. This is a plain-text heuristic, not a diff parser —
// good enough for giving the LLM a relevant excerpt, not for anything that
// needs exact hunk boundaries.
func filterDiffForFile(diff, path string) string {
	gitHeaderPrefix := "diff --git a/" + path + " b/" + path
	untrackedPrefix := "=== untracked file: " + path + " ==="

	var b strings.Builder
	capturing := false
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git a/"):
			capturing = strings.HasPrefix(line, gitHeaderPrefix)
		case strings.HasPrefix(line, "=== untracked file: "):
			capturing = line == untrackedPrefix
		case strings.HasPrefix(line, "=== unstaged changes") || strings.HasPrefix(line, "=== staged changes"):
			capturing = false
		}
		if capturing {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}
