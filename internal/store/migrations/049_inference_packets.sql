-- +goose Up
CREATE TABLE IF NOT EXISTS inference_packets (
    id           TEXT PRIMARY KEY,
    context_id   TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    session_id   TEXT REFERENCES sync_sessions(id) ON DELETE SET NULL,
    generated_at TEXT NOT NULL,
    packet_json  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_inference_packets_context ON inference_packets(context_id, generated_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_inference_packets_context;
DROP TABLE IF EXISTS inference_packets;
