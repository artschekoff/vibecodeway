-- +goose Up
CREATE TABLE IF NOT EXISTS ai_prompts (
    id           TEXT PRIMARY KEY,
    label        TEXT NOT NULL,
    body         TEXT NOT NULL,
    default_body TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);

INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'event_extraction',
'Event Extraction',
'You are an AI analyst extracting structured events from project communication data.

## Context
Project/module key: {{context_key}}

## Source data
{{raw_content}}{{user_next_steps}}

## Task
Extract a JSON array of events. Each event represents a distinct, meaningful fact.
Skip noise (greetings, off-topic messages, duplicate information).

## Event types (use exactly these strings):
- requirement_stated      — official requirement from Jira or formal doc
- requirement_confirmed   — informal requirement confirmed by authorised person
- requirement_rejected    — requirement explicitly cancelled
- open_question_raised    — unresolved question or ambiguity
- open_question_answered  — previously open question answered
- decision_made           — technical or business decision taken
- blocker_identified      — blocker or problem found
- blocker_resolved        — blocker removed
- work_completed          — task/step done (PR merged, ticket closed, confirmed in chat)
- status_update           — general progress update
- next_step_recommendation — explicit recommendation about what should happen next
- user_note               — contextual note from a team member worth preserving

## Source types:
jira | slack | github | chat | user

## Output format — return ONLY valid JSON, no markdown fences:
[
  {
    "type": "requirement_stated",
    "timestamp": "2024-01-15T10:30:00Z",
    "source_type": "jira",
    "source_id": "PROJ-123",
    "author": "username",
    "summary": "One or two sentence summary of what happened.",
    "raw_reference": "Original text or URL"
  }
]',
'You are an AI analyst extracting structured events from project communication data.

## Context
Project/module key: {{context_key}}

## Source data
{{raw_content}}{{user_next_steps}}

## Task
Extract a JSON array of events. Each event represents a distinct, meaningful fact.
Skip noise (greetings, off-topic messages, duplicate information).

## Event types (use exactly these strings):
- requirement_stated      — official requirement from Jira or formal doc
- requirement_confirmed   — informal requirement confirmed by authorised person
- requirement_rejected    — requirement explicitly cancelled
- open_question_raised    — unresolved question or ambiguity
- open_question_answered  — previously open question answered
- decision_made           — technical or business decision taken
- blocker_identified      — blocker or problem found
- blocker_resolved        — blocker removed
- work_completed          — task/step done (PR merged, ticket closed, confirmed in chat)
- status_update           — general progress update
- next_step_recommendation — explicit recommendation about what should happen next
- user_note               — contextual note from a team member worth preserving

## Source types:
jira | slack | github | chat | user

## Output format — return ONLY valid JSON, no markdown fences:
[
  {
    "type": "requirement_stated",
    "timestamp": "2024-01-15T10:30:00Z",
    "source_type": "jira",
    "source_id": "PROJ-123",
    "author": "username",
    "summary": "One or two sentence summary of what happened.",
    "raw_reference": "Original text or URL"
  }
]',
'2026-01-01T00:00:00Z'
);

INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'snapshot_build',
'Snapshot Build',
'You are an AI analyst maintaining a structured project snapshot.

## Context key: {{context_key}}

## Full event log (chronological)
{{events}}

## User-provided next-step hints
Short term (~1 day): {{next_steps_short}}
Mid term (~1 week): {{next_steps_mid}}
Long term (~1 month): {{next_steps_long}}

## Task
Produce a JSON object with these exact fields. All sections are Markdown.

{
  "current_step": {
    "description": "What is being worked on RIGHT NOW (1-2 sentences)",
    "confidence": "high|medium|low",
    "from_event_ids": ["event-id-1", "event-id-2"]
  },
  "status": "not_started|in_progress|scoping|needs_attention|ready",
  "section_operational": "## Current status\n...\n## Active blockers\n...\n## Open questions\n...\n## Who is doing what\n...",
  "section_narrative": "## History\n...\n## Decisions and rationale\n...\n## What was tried\n...\n## Domain entities\n...\n## Constraints\n...",
  "section_engineering": "## Technical entities\n...\n## Relevant modules\n...\n## Active branch and PRs\n...\n## Known issues\n...\n## Architectural constraints\n...",
  "ai_next_steps_short": "Concrete actions for the next ~1 day",
  "ai_next_steps_mid": "Goals for the next ~1 week",
  "ai_next_steps_long": "Strategic direction for the next ~1 month"
}

Rules:
- status "needs_attention" wins over all other statuses if there are open blockers or CI failures
- status "scoping" if there are unresolved open questions with no blocker
- Incorporate user-provided next-step hints when generating ai_next_steps_* fields
- Return ONLY valid JSON, no markdown fences',
'You are an AI analyst maintaining a structured project snapshot.

## Context key: {{context_key}}

## Full event log (chronological)
{{events}}

## User-provided next-step hints
Short term (~1 day): {{next_steps_short}}
Mid term (~1 week): {{next_steps_mid}}
Long term (~1 month): {{next_steps_long}}

## Task
Produce a JSON object with these exact fields. All sections are Markdown.

{
  "current_step": {
    "description": "What is being worked on RIGHT NOW (1-2 sentences)",
    "confidence": "high|medium|low",
    "from_event_ids": ["event-id-1", "event-id-2"]
  },
  "status": "not_started|in_progress|scoping|needs_attention|ready",
  "section_operational": "## Current status\n...\n## Active blockers\n...\n## Open questions\n...\n## Who is doing what\n...",
  "section_narrative": "## History\n...\n## Decisions and rationale\n...\n## What was tried\n...\n## Domain entities\n...\n## Constraints\n...",
  "section_engineering": "## Technical entities\n...\n## Relevant modules\n...\n## Active branch and PRs\n...\n## Known issues\n...\n## Architectural constraints\n...",
  "ai_next_steps_short": "Concrete actions for the next ~1 day",
  "ai_next_steps_mid": "Goals for the next ~1 week",
  "ai_next_steps_long": "Strategic direction for the next ~1 month"
}

Rules:
- status "needs_attention" wins over all other statuses if there are open blockers or CI failures
- status "scoping" if there are unresolved open questions with no blocker
- Incorporate user-provided next-step hints when generating ai_next_steps_* fields
- Return ONLY valid JSON, no markdown fences',
'2026-01-01T00:00:00Z'
);

INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'handoff_packet',
'Handoff Packet',
'You are an expert business analyst extracting structured business requirements from project communication logs.
Do NOT reference code files, repositories, function names, or database schema — focus entirely on BUSINESS requirements, rules, goals, and decisions.

## Context key: {{context_key}}

## Full event log (chronological)
{{events}}

## Task
Produce a JSON object with exactly this structure. Return ONLY valid JSON, no markdown fences.

{
  "summary": {
    "product_or_initiative": "Name of the product or initiative (string or null)",
    "business_problem": "The main business problem being solved (string or null)",
    "target_users": ["user role or stakeholder group"],
    "desired_outcomes": ["business outcome the work optimizes for"],
    "non_goals": ["things explicitly out of scope"],
    "inferred_tradeoffs": ["e.g. speed over completeness"],
    "global_confidence": 0.85
  },
  "inferred_requirements": [
    {
      "requirement_id": "req-001",
      "requirement_type": "goal|business_rule|functional_requirement|acceptance_criterion|constraint|non_goal|dependency|assumption|risk|open_question",
      "title": "Short title",
      "statement": "Clear, self-contained business statement written as a fact, need, rule, criterion, risk, or question.",
      "status": "inferred|corroborated|conflicting|stale|rejected",
      "priority": "low|medium|high|critical",
      "confidence": 0.9,
      "ambiguity_notes": null,
      "business_value": "why this matters from a business perspective (string or null)",
      "source_artifacts": [
        {
          "artifact_id": "PROJ-123 or slack-thread-id",
          "artifact_type": "task|comment|chat_message|issue|pull_request|document|other",
          "source_system": "jira|slack|github|user",
          "title": "Short human-readable label (string or null)",
          "author": "username (string or null)"
        }
      ],
      "evidence": [
        {
          "artifact_id": "must match a source_artifacts artifact_id above",
          "quote": "verbatim excerpt or distilled statement from the source",
          "rationale": "why this quote supports the requirement (string or null)"
        }
      ],
      "affected_entities": [
        {
          "entity_type": "feature|workflow|user_role|integration|report|dashboard|other",
          "entity_id": "slug-identifier",
          "label": "Human-readable name (string or null)"
        }
      ],
      "relations": [
        {
          "relation_type": "supports|refines|depends_on|conflicts_with|duplicates|derived_from|supersedes|affects",
          "target_requirement_id": "req-002",
          "reason": "brief explanation (string or null)"
        }
      ],
      "valid_from": null,
      "last_seen_at": null,
      "superseded_by": null
    }
  ],
  "handoff_candidate": {
    "handoff_title": "Short title of the business task",
    "objective": "Concrete business objective a downstream team should satisfy",
    "in_scope_requirements": ["req-001"],
    "out_of_scope_requirements": [],
    "key_business_rules": ["Mandatory business rule from the requirements"],
    "acceptance_criteria": ["Business-facing acceptance criterion that defines success"],
    "open_questions": ["Unresolved question that may block confident implementation"],
    "confidence": 0.8
  },
  "extraction_notes": null
}

Rules:
- Use req-001, req-002, ... as requirement_id — unique within this packet, used consistently in relations.
- Only cite artifact_id values that appear in the event log above. Never invent IDs.
- Set handoff_candidate to null if there are too many open questions or the context is still scoping.
- Do not mention code files, function names, class names, or database fields in any field.
- All arrays must be present (use [] if empty). Nullable fields use null.
- Return ONLY the JSON object — no other text.',
'You are an expert business analyst extracting structured business requirements from project communication logs.
Do NOT reference code files, repositories, function names, or database schema — focus entirely on BUSINESS requirements, rules, goals, and decisions.

## Context key: {{context_key}}

## Full event log (chronological)
{{events}}

## Task
Produce a JSON object with exactly this structure. Return ONLY valid JSON, no markdown fences.

{
  "summary": {
    "product_or_initiative": "Name of the product or initiative (string or null)",
    "business_problem": "The main business problem being solved (string or null)",
    "target_users": ["user role or stakeholder group"],
    "desired_outcomes": ["business outcome the work optimizes for"],
    "non_goals": ["things explicitly out of scope"],
    "inferred_tradeoffs": ["e.g. speed over completeness"],
    "global_confidence": 0.85
  },
  "inferred_requirements": [
    {
      "requirement_id": "req-001",
      "requirement_type": "goal|business_rule|functional_requirement|acceptance_criterion|constraint|non_goal|dependency|assumption|risk|open_question",
      "title": "Short title",
      "statement": "Clear, self-contained business statement written as a fact, need, rule, criterion, risk, or question.",
      "status": "inferred|corroborated|conflicting|stale|rejected",
      "priority": "low|medium|high|critical",
      "confidence": 0.9,
      "ambiguity_notes": null,
      "business_value": "why this matters from a business perspective (string or null)",
      "source_artifacts": [
        {
          "artifact_id": "PROJ-123 or slack-thread-id",
          "artifact_type": "task|comment|chat_message|issue|pull_request|document|other",
          "source_system": "jira|slack|github|user",
          "title": "Short human-readable label (string or null)",
          "author": "username (string or null)"
        }
      ],
      "evidence": [
        {
          "artifact_id": "must match a source_artifacts artifact_id above",
          "quote": "verbatim excerpt or distilled statement from the source",
          "rationale": "why this quote supports the requirement (string or null)"
        }
      ],
      "affected_entities": [
        {
          "entity_type": "feature|workflow|user_role|integration|report|dashboard|other",
          "entity_id": "slug-identifier",
          "label": "Human-readable name (string or null)"
        }
      ],
      "relations": [
        {
          "relation_type": "supports|refines|depends_on|conflicts_with|duplicates|derived_from|supersedes|affects",
          "target_requirement_id": "req-002",
          "reason": "brief explanation (string or null)"
        }
      ],
      "valid_from": null,
      "last_seen_at": null,
      "superseded_by": null
    }
  ],
  "handoff_candidate": {
    "handoff_title": "Short title of the business task",
    "objective": "Concrete business objective a downstream team should satisfy",
    "in_scope_requirements": ["req-001"],
    "out_of_scope_requirements": [],
    "key_business_rules": ["Mandatory business rule from the requirements"],
    "acceptance_criteria": ["Business-facing acceptance criterion that defines success"],
    "open_questions": ["Unresolved question that may block confident implementation"],
    "confidence": 0.8
  },
  "extraction_notes": null
}

Rules:
- Use req-001, req-002, ... as requirement_id — unique within this packet, used consistently in relations.
- Only cite artifact_id values that appear in the event log above. Never invent IDs.
- Set handoff_candidate to null if there are too many open questions or the context is still scoping.
- Do not mention code files, function names, class names, or database fields in any field.
- All arrays must be present (use [] if empty). Nullable fields use null.
- Return ONLY the JSON object — no other text.',
'2026-01-01T00:00:00Z'
);

-- +goose Down
DROP TABLE IF EXISTS ai_prompts;
