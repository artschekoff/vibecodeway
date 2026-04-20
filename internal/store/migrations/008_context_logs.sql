-- +goose Up

-- context_logs stores one AI-generated (or manually authored) summary per context change event.
-- trigger:     'refresh' | 'reload' | 'manual' | 'chat' | 'init'
-- is_manual:   1 when created or last edited by the user
-- diff_json:   structured diff payload (added/removed/changed object keys) captured at record time;
--              optional for manual entries
CREATE TABLE IF NOT EXISTS context_logs (
    id                TEXT PRIMARY KEY,
    context_id        TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    trigger           TEXT NOT NULL,
    source            TEXT NOT NULL DEFAULT '',
    model             TEXT NOT NULL DEFAULT '',
    prompt_tokens     INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    diff_json         TEXT NOT NULL DEFAULT '',
    summary_md        TEXT NOT NULL DEFAULT '',
    is_manual         INTEGER NOT NULL DEFAULT 0,
    edited_at         TEXT,
    created_at        TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_context_logs_context_id_created_at
    ON context_logs(context_id, created_at DESC);

-- consolidated_context_logs keeps a single rolling AI-maintained summary per context.
-- updated_by: 'ai' | 'user' (last writer)
CREATE TABLE IF NOT EXISTS consolidated_context_logs (
    context_id TEXT PRIMARY KEY REFERENCES contexts(id) ON DELETE CASCADE,
    content_md TEXT NOT NULL DEFAULT '',
    version    INTEGER NOT NULL DEFAULT 1,
    updated_by TEXT NOT NULL DEFAULT 'ai',
    updated_at TEXT NOT NULL
);

-- context_log_chat_messages holds the AI conversation transcript per context.
-- role: 'user' | 'assistant' | 'system'
-- proposed_consolidated_md: non-null when the assistant proposed a full rewrite the user can Apply.
-- applied_at: set when the proposal has been applied to consolidated_context_logs.
CREATE TABLE IF NOT EXISTS context_log_chat_messages (
    id                       TEXT PRIMARY KEY,
    context_id               TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    role                     TEXT NOT NULL,
    content                  TEXT NOT NULL DEFAULT '',
    proposed_consolidated_md TEXT,
    applied_at               TEXT,
    created_at               TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_context_log_chat_context_id_created_at
    ON context_log_chat_messages(context_id, created_at ASC);

-- ai_settings is a singleton row (id = 1) storing OpenAI configuration.
-- openai_key_enc / openai_key_nonce: AES-GCM ciphertext of the API key, encrypted with a KEK
-- derived from the Slack passphrase. Both NULL when the user has not stored a key in-app
-- (env var OPENAI_API_KEY takes priority anyway).
CREATE TABLE IF NOT EXISTS ai_settings (
    id               INTEGER PRIMARY KEY,
    openai_key_enc   BLOB,
    openai_key_nonce BLOB,
    model            TEXT NOT NULL DEFAULT 'gpt-4o-mini',
    base_url         TEXT NOT NULL DEFAULT '',
    updated_at       TEXT NOT NULL
);

-- +goose Down

DROP INDEX IF EXISTS idx_context_log_chat_context_id_created_at;
DROP INDEX IF EXISTS idx_context_logs_context_id_created_at;
DROP TABLE IF EXISTS ai_settings;
DROP TABLE IF EXISTS context_log_chat_messages;
DROP TABLE IF EXISTS consolidated_context_logs;
DROP TABLE IF EXISTS context_logs;
