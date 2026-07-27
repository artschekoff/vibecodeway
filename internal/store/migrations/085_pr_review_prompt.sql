-- +goose Up

-- Seed the "pr_review" prompt used by BuildReviewTask to assemble a PR-review
-- instruction for an external coding agent. Pure string assembly (no LLM call).

INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'pr_review',
'PR Review',
'/arts-load-context {{context_key}}

You are a senior code reviewer. Review pull request #{{pr_number}} — {{pr_title}}.

The PR branch is checked out as a git worktree below. Diff it against its base branch (e.g. `git diff main...HEAD` inside the worktree), then review the changed files for correctness bugs, security issues, edge cases, and maintainability.

Do NOT modify the code. Produce a review: list findings as file:line — severity — issue — suggested fix, then a short overall assessment.

## Worktrees on disk (the PR branch is checked out here)
{{repos}}',
'/arts-load-context {{context_key}}

You are a senior code reviewer. Review pull request #{{pr_number}} — {{pr_title}}.

The PR branch is checked out as a git worktree below. Diff it against its base branch (e.g. `git diff main...HEAD` inside the worktree), then review the changed files for correctness bugs, security issues, edge cases, and maintainability.

Do NOT modify the code. Produce a review: list findings as file:line — severity — issue — suggested fix, then a short overall assessment.

## Worktrees on disk (the PR branch is checked out here)
{{repos}}',
'2026-07-27T00:00:00Z'
);

-- +goose Down

DELETE FROM ai_prompts WHERE id='pr_review';
