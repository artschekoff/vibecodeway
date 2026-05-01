-- +goose Up
INSERT INTO organizations (id, name, sort_order, created_at, updated_at)
VALUES (
  'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
  'JIRO',
  1,
  datetime('now'),
  datetime('now')
);

UPDATE contexts
SET org_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'
WHERE org_id IS NULL;

-- +goose Down
UPDATE contexts
SET org_id = NULL
WHERE org_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';

DELETE FROM organizations WHERE id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
