-- +goose Up

-- Seed the "branch_match" prompt used by MatchBranchesToContext to match Git
-- branch NAMES (titles only — no code/diffs/bodies) to a context's ticket
-- key(s)/feature/subject. INSERT OR IGNORE keeps this safe if a row already exists.

INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'branch_match',
'Branch Match',
'You match Git branch names to a work context using ONLY the branch names below — no other information exists or should be inferred.

## Context key
{{context_key}}

## Context summary (tickets and focus)
{{context_summary}}

## Candidate branch names (index: name)
{{branch_titles}}

A branch belongs to this context if its name references the context''s ticket key(s), feature, or subject. Be precise — do NOT match generic branches (chore/deps, release, hotfix) unless clearly tied to this context.

Respond with ONLY a JSON array of the 0-based indices of matching branches (e.g. [0, 3]). If none match, return [].',
'You match Git branch names to a work context using ONLY the branch names below — no other information exists or should be inferred.

## Context key
{{context_key}}

## Context summary (tickets and focus)
{{context_summary}}

## Candidate branch names (index: name)
{{branch_titles}}

A branch belongs to this context if its name references the context''s ticket key(s), feature, or subject. Be precise — do NOT match generic branches (chore/deps, release, hotfix) unless clearly tied to this context.

Respond with ONLY a JSON array of the 0-based indices of matching branches (e.g. [0, 3]). If none match, return [].',
'2026-01-01T00:00:00Z'
);

-- +goose Down

DELETE FROM ai_prompts WHERE id='branch_match';
