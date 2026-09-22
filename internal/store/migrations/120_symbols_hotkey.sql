-- +goose Up
-- Registry id must match frontend/src/hotkeys/registry.ts (symbols.search ⌘⇧O).
INSERT INTO keybindings
  (id, command, label, description, modifier_keys, key_code, sequence, object, action, scope,
   default_modifier_keys, default_key_code, default_sequence, updated_at)
VALUES
  ('kb-symbols-search','symbols.search','Search symbols','Fuzzy-search functions, classes and variables in the active repo','cmd,shift','o','','symbols','search','editor','cmd,shift','o','','');

-- +goose Down
DELETE FROM keybindings WHERE id = 'kb-symbols-search';
