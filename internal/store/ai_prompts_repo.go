package store

import (
	"context"
	"fmt"
	"time"
)

// knownPlaceholders defines the available {{placeholder}} variables for each prompt.
var knownPlaceholders = map[string][]PromptPlaceholder{
	"event_extraction": {
		{Name: "context_key", Description: "Project/module key (e.g. JH-2210)"},
		{Name: "raw_content", Description: "Raw formatted content from all providers (Jira, Slack, GitHub)"},
		{Name: "user_next_steps", Description: "User-provided next-step hints block (empty if none set)"},
	},
	"snapshot_build": {
		{Name: "context_key", Description: "Project/module key"},
		{Name: "events", Description: "Full event log as JSON array"},
		{Name: "next_steps_short", Description: "User short-term hints (~1 day)"},
		{Name: "next_steps_mid", Description: "User mid-term hints (~1 week)"},
		{Name: "next_steps_long", Description: "User long-term hints (~1 month)"},
		{Name: "status_definitions", Description: "Available workflow statuses with AI hints — injected automatically"},
		{Name: "current_status_log_entry", Description: "Latest status log entry (current status + reason) — injected automatically"},
	},
	"handoff_packet": {
		{Name: "context_key", Description: "Project/module key"},
		{Name: "events", Description: "Full event log as JSON array"},
	},
	"person_assignment": {
		{Name: "context_key", Description: "Project/module key of the context"},
		{Name: "raw_content", Description: "Formatted Jira issues and activity for this context"},
		{Name: "events", Description: "Full event log in chronological order"},
		{Name: "custom_instructions", Description: "Optional extra instructions section (empty by default)"},
	},
	"person_assignment_with_code": {
		{Name: "context_key", Description: "Project/module key of the context"},
		{Name: "raw_content", Description: "Formatted Jira issues and activity for this context"},
		{Name: "events", Description: "Full event log in chronological order"},
		{Name: "custom_instructions", Description: "Optional extra instructions section (empty by default)"},
		{Name: "codebase_context", Description: "Relevant source files from the linked repository, injected for grounding"},
	},
	"codebase_file_select": {
		{Name: "task_summary", Description: "Short summary of the task the file selection is being made for"},
		{Name: "file_tree", Description: "Repo-relative file tree listing to choose from"},
	},
	"person_assignment_refine": {
		{Name: "person_name", Description: "The name of the person whose assignment is being refined"},
		{Name: "current_body", Description: "The current Markdown body for this person"},
		{Name: "user_instruction", Description: "The user's modification instruction"},
		{Name: "raw_content", Description: "Formatted Jira issues and activity (reference)"},
		{Name: "events", Description: "Full event log (reference)"},
	},
	"person_assignment_summary": {
		{Name: "body", Description: "The full task assignment body to compress"},
	},
	"person_assignment_judge": {
		{Name: "output_state", Description: "The updated assignment being evaluated"},
		{Name: "message_content", Description: "The message that triggered the update"},
	},
	"ask_ai": {
		// The Ask AI agent system prompt takes no placeholders; it drives tool use.
	},
	"context_updater": {
		{Name: "context_key", Description: "Project/module key of the context"},
		{Name: "current_status", Description: "Current workflow status slug — injected automatically"},
		{Name: "workflow_statuses", Description: "Available workflow status slugs with labels — injected automatically"},
		{Name: "existing_repos", Description: "VCS repos already linked to this context, for dedup — injected automatically (best-effort, empty if unavailable)"},
		{Name: "existing_tickets", Description: "Jira issue keys already linked to this context, for dedup — injected automatically (best-effort, empty if unavailable)"},
		{Name: "user_text", Description: "The user's free-form Quick Update note"},
	},
	"plan_generation": {
		{Name: "context_key", Description: "Project/module key of the context"},
		{Name: "assignment_body", Description: "The person assignment's Markdown body the plan is generated for"},
		{Name: "raw_content", Description: "Formatted Jira issues and activity for this context"},
		{Name: "codebase_context", Description: "Relevant source files from the linked repository, injected for grounding (empty if no repo linked)"},
	},
	"branch_match": {
		{Name: "context_key", Description: "Project/module key of the context"},
		{Name: "context_summary", Description: "Context summary — tickets and focus — used to judge relevance"},
		{Name: "branch_titles", Description: "Numbered list of candidate branch names (index: name), titles only"},
	},
	"agent_task": {
		{Name: "user_prompt", Description: "The user's free-form task instruction for the coding agent"},
		{Name: "context_key", Description: "Project/module key of the context"},
		{Name: "tickets", Description: "Titles-only ticket lines (key + summary) linked to this context"},
		{Name: "repos", Description: "On-disk local paths (and branches) of repos attached to this context"},
	},
}

// ListAIPrompts returns all prompt templates with their placeholder definitions.
func (s *Store) ListAIPrompts(ctx context.Context) ([]AIPrompt, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, label, body, default_body, updated_at
		FROM ai_prompts ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("list ai prompts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []AIPrompt
	for rows.Next() {
		var p AIPrompt
		if err := rows.Scan(&p.ID, &p.Label, &p.Body, &p.DefaultBody, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan ai prompt: %w", err)
		}
		p.Placeholders = knownPlaceholders[p.ID]
		result = append(result, p)
	}
	return result, rows.Err()
}

// GetAIPrompt returns a single prompt by ID with its placeholders populated.
func (s *Store) GetAIPrompt(ctx context.Context, id string) (*AIPrompt, error) {
	var p AIPrompt
	err := s.db.QueryRowContext(ctx, `
		SELECT id, label, body, default_body, updated_at FROM ai_prompts WHERE id = ?
	`, id).Scan(&p.ID, &p.Label, &p.Body, &p.DefaultBody, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get ai prompt %q: %w", id, err)
	}
	p.Placeholders = knownPlaceholders[p.ID]
	return &p, nil
}

// UpdateAIPrompt replaces the body of an existing prompt.
func (s *Store) UpdateAIPrompt(ctx context.Context, id, body string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE ai_prompts SET body = ?, updated_at = ? WHERE id = ?
	`, body, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("update ai prompt %q: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("ai prompt %q not found", id)
	}
	return nil
}

// ResetAIPrompt restores the prompt body to its seeded default.
func (s *Store) ResetAIPrompt(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE ai_prompts SET body = default_body, updated_at = ? WHERE id = ?
	`, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("reset ai prompt %q: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("ai prompt %q not found", id)
	}
	return nil
}
