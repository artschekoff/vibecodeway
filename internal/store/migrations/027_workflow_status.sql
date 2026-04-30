-- +goose Up

-- AI-computed values, updated on every refresh.
ALTER TABLE contexts ADD COLUMN ai_proposed_status   TEXT NOT NULL DEFAULT 'not_started';
ALTER TABLE contexts ADD COLUMN ai_proposed_comment  TEXT;
ALTER TABLE contexts ADD COLUMN ai_proposed_next_step TEXT;
ALTER TABLE contexts ADD COLUMN ai_proposed_at       TEXT;

-- User overrides — NULL means "accept AI proposal".
ALTER TABLE contexts ADD COLUMN user_status_override    TEXT;
ALTER TABLE contexts ADD COLUMN user_comment_override   TEXT;
ALTER TABLE contexts ADD COLUMN user_next_step_override TEXT;
ALTER TABLE contexts ADD COLUMN user_override_updated_at TEXT;

-- +goose Down

ALTER TABLE contexts DROP COLUMN ai_proposed_status;
ALTER TABLE contexts DROP COLUMN ai_proposed_comment;
ALTER TABLE contexts DROP COLUMN ai_proposed_next_step;
ALTER TABLE contexts DROP COLUMN ai_proposed_at;
ALTER TABLE contexts DROP COLUMN user_status_override;
ALTER TABLE contexts DROP COLUMN user_comment_override;
ALTER TABLE contexts DROP COLUMN user_next_step_override;
ALTER TABLE contexts DROP COLUMN user_override_updated_at;
