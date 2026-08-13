-- +goose Up
-- Retune the switch chords:
--   context tabs   ⌘] / ⌘[   -> ⌘⇧] / ⌘⇧[
--   terminal       ⌘⇧] / ⌘⇧[ -> ⌘] / ⌘[
--   sidebar FILES/GIT toggle (⌘⇧]/⌘⇧[) -> direct ⌘B F / ⌘B G
-- Live bindings move only when still on the old shipped default (a user's custom
-- binding is left alone); default_* columns always move to the new shipped default.

-- Context tabs: cmd -> cmd,shift (key_code unchanged)
UPDATE keybindings SET modifier_keys = 'cmd,shift'
  WHERE command = 'tabs.next' AND modifier_keys = 'cmd' AND key_code = ']';
UPDATE keybindings SET modifier_keys = 'cmd,shift'
  WHERE command = 'tabs.prev' AND modifier_keys = 'cmd' AND key_code = '[';
UPDATE keybindings SET default_modifier_keys = 'cmd,shift'
  WHERE command IN ('tabs.next', 'tabs.prev');

-- Terminal switch: cmd,shift -> cmd (key_code unchanged)
UPDATE keybindings SET modifier_keys = 'cmd'
  WHERE command = 'terminal.next' AND modifier_keys = 'cmd,shift' AND key_code = ']';
UPDATE keybindings SET modifier_keys = 'cmd'
  WHERE command = 'terminal.prev' AND modifier_keys = 'cmd,shift' AND key_code = '[';
UPDATE keybindings SET default_modifier_keys = 'cmd'
  WHERE command IN ('terminal.next', 'terminal.prev');

-- Sidebar: drop the next/prev toggle, add direct ⌘B F (files) / ⌘B G (git).
DELETE FROM keybindings WHERE command IN ('sidebar.next', 'sidebar.prev');
INSERT INTO keybindings
  (id, command, label, description, modifier_keys, key_code, sequence, object, action, scope,
   default_modifier_keys, default_key_code, default_sequence, updated_at)
VALUES
  ('kb-sidebar-files','sidebar.files','Show FILES','Show the FILES sidebar tab','cmd','b','f','sidebar','files','editor','cmd','b','f',''),
  ('kb-sidebar-git','sidebar.git','Show GIT','Show the GIT sidebar tab','cmd','b','g','sidebar','git','editor','cmd','b','g','')
ON CONFLICT(command) DO NOTHING;

-- +goose Down
UPDATE keybindings SET modifier_keys = 'cmd', default_modifier_keys = 'cmd'
  WHERE command = 'tabs.next' AND key_code = ']';
UPDATE keybindings SET modifier_keys = 'cmd', default_modifier_keys = 'cmd'
  WHERE command = 'tabs.prev' AND key_code = '[';
UPDATE keybindings SET modifier_keys = 'cmd,shift', default_modifier_keys = 'cmd,shift'
  WHERE command = 'terminal.next' AND key_code = ']';
UPDATE keybindings SET modifier_keys = 'cmd,shift', default_modifier_keys = 'cmd,shift'
  WHERE command = 'terminal.prev' AND key_code = '[';
DELETE FROM keybindings WHERE command IN ('sidebar.files', 'sidebar.git');
INSERT INTO keybindings
  (id, command, label, description, modifier_keys, key_code, sequence, object, action, scope,
   default_modifier_keys, default_key_code, default_sequence, updated_at)
VALUES
  ('kb-sidebar-next','sidebar.next','Next sidebar tab','Switch FILES / GIT (forward)','cmd,shift',']','','sidebar','next','editor','cmd,shift',']','',''),
  ('kb-sidebar-prev','sidebar.prev','Previous sidebar tab','Switch FILES / GIT (back)','cmd,shift','[','','sidebar','prev','editor','cmd,shift','[','','')
ON CONFLICT(command) DO NOTHING;
