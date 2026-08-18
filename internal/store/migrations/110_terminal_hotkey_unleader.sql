-- +goose Up
-- ⌘T was the lead of a sequence (⌘T H = here, ⌘T I = insert), so every ⌘T sat in
-- the 600ms leader window before a terminal appeared and read as "didn't fire".
-- Give "here" and "insert" their own chords so ⌘T is a lone bare binding:
--   terminal.here   ⌘T H -> ⌘⇧T
--   terminal.insert ⌘T I -> ⌘I
-- Live bindings move only when still on the old shipped default (a user's custom
-- binding is left alone); default_* columns always move to the new shipped default.

UPDATE keybindings SET modifier_keys = 'cmd,shift', key_code = 't', sequence = ''
  WHERE command = 'terminal.here' AND modifier_keys = 'cmd' AND key_code = 't' AND sequence = 'h';
UPDATE keybindings SET default_modifier_keys = 'cmd,shift', default_key_code = 't', default_sequence = ''
  WHERE command = 'terminal.here';

UPDATE keybindings SET modifier_keys = 'cmd', key_code = 'i', sequence = ''
  WHERE command = 'terminal.insert' AND modifier_keys = 'cmd' AND key_code = 't' AND sequence = 'i';
UPDATE keybindings SET default_modifier_keys = 'cmd', default_key_code = 'i', default_sequence = ''
  WHERE command = 'terminal.insert';

-- +goose Down
UPDATE keybindings SET modifier_keys = 'cmd', key_code = 't', sequence = 'h',
       default_modifier_keys = 'cmd', default_key_code = 't', default_sequence = 'h'
  WHERE command = 'terminal.here';
UPDATE keybindings SET modifier_keys = 'cmd', key_code = 't', sequence = 'i',
       default_modifier_keys = 'cmd', default_key_code = 't', default_sequence = 'i'
  WHERE command = 'terminal.insert';
