-- +goose Up

-- Seed the "plan_generation" prompt used by GeneratePlan to produce a
-- writing-plans-style implementation plan markdown document for a single
-- person assignment, grounded in the assignment body, the context's raw
-- content, and (when a repo is linked) Codebase Read Mode output.
-- INSERT OR IGNORE keeps this safe if a row already exists.

INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'plan_generation',
'Plan Generation',
'You are a senior engineer writing a precise, writing-plans-style implementation plan. Ground every step in the assignment, the context, and the shown codebase. Use concrete file paths and inline `path:line` references from the Codebase context when present. Output GitHub-flavored markdown only.

## Context key
{{context_key}}

## Assignment
{{assignment_body}}

## Raw context content
{{raw_content}}
{{codebase_context}}
## Task
Write a step-by-step implementation plan for the assignment above. The plan must:
- Be grounded in the assignment, the raw context content, and the codebase context — do not invent requirements not supported by them
- Use concrete file paths and inline `path:line` references from the Codebase context section when it is non-empty
- Break the work into small, ordered, independently verifiable steps
- Call out open questions or blockers as a separate section when relevant

Output ONLY the plan as GitHub-flavored markdown — no preamble, no trailing commentary.',
'You are a senior engineer writing a precise, writing-plans-style implementation plan. Ground every step in the assignment, the context, and the shown codebase. Use concrete file paths and inline `path:line` references from the Codebase context when present. Output GitHub-flavored markdown only.

## Context key
{{context_key}}

## Assignment
{{assignment_body}}

## Raw context content
{{raw_content}}
{{codebase_context}}
## Task
Write a step-by-step implementation plan for the assignment above. The plan must:
- Be grounded in the assignment, the raw context content, and the codebase context — do not invent requirements not supported by them
- Use concrete file paths and inline `path:line` references from the Codebase context section when it is non-empty
- Break the work into small, ordered, independently verifiable steps
- Call out open questions or blockers as a separate section when relevant

Output ONLY the plan as GitHub-flavored markdown — no preamble, no trailing commentary.',
'2026-01-01T00:00:00Z'
);

-- +goose Down

DELETE FROM ai_prompts WHERE id='plan_generation';
