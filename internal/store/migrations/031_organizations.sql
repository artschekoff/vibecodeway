-- +goose Up
CREATE TABLE organizations (
  id         TEXT PRIMARY KEY,
  name       TEXT UNIQUE NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

ALTER TABLE contexts ADD COLUMN org_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
