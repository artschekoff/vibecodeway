-- +goose Up
-- Seed the ⌘⇧W "close active context tab" hotkey. modifier_keys uses the neutral
-- token "cmd" which the frontend resolves to the platform primary (mod).
INSERT INTO keybindings
  (id, command, label, description, modifier_keys, key_code, sequence, object, action, scope,
   default_modifier_keys, default_key_code, default_sequence, updated_at)
VALUES
  ('kb-tabs-close','tabs.close','Close context tab','Close the active context tab','cmd,shift','w','','tabs','close','global','cmd,shift','w','','')
ON CONFLICT(command) DO NOTHING;

-- +goose Down
DELETE FROM keybindings WHERE command = 'tabs.close';
