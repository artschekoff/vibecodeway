-- +goose Up

-- Add multi-provider support to ai_settings.
-- provider: which LLM provider to use ("openai" | "anthropic" | "local").
-- anthropic_key_enc / anthropic_key_nonce: AES-256-GCM encrypted Anthropic API key.
-- Existing openai_key_enc / openai_key_nonce are kept for OpenAI and local providers.
ALTER TABLE ai_settings ADD COLUMN provider TEXT NOT NULL DEFAULT 'openai';
ALTER TABLE ai_settings ADD COLUMN anthropic_key_enc   BLOB;
ALTER TABLE ai_settings ADD COLUMN anthropic_key_nonce BLOB;

-- +goose Down

ALTER TABLE ai_settings DROP COLUMN provider;
ALTER TABLE ai_settings DROP COLUMN anthropic_key_enc;
ALTER TABLE ai_settings DROP COLUMN anthropic_key_nonce;
