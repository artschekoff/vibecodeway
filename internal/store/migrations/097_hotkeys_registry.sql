-- +goose Up
ALTER TABLE keybindings ADD COLUMN object                TEXT NOT NULL DEFAULT '';
ALTER TABLE keybindings ADD COLUMN action                TEXT NOT NULL DEFAULT '';
ALTER TABLE keybindings ADD COLUMN scope                 TEXT NOT NULL DEFAULT 'global';
ALTER TABLE keybindings ADD COLUMN sequence              TEXT NOT NULL DEFAULT '';
ALTER TABLE keybindings ADD COLUMN default_modifier_keys TEXT NOT NULL DEFAULT '';
ALTER TABLE keybindings ADD COLUMN default_key_code      TEXT NOT NULL DEFAULT '';
ALTER TABLE keybindings ADD COLUMN default_sequence      TEXT NOT NULL DEFAULT '';

-- Seed all in-app hotkeys. command == registry id. modifier_keys uses the neutral
-- token "cmd" which the frontend resolves to the platform primary (mod). Zone-focus
-- commands use "ctrl" verbatim (raw control on every platform).
INSERT INTO keybindings
  (id, command, label, description, modifier_keys, key_code, sequence, object, action, scope,
   default_modifier_keys, default_key_code, default_sequence, updated_at)
VALUES
  ('kb-search-global','search.global','Global search','Search contexts across all organizations','cmd','f','','search','global','global','cmd','f','',''),
  ('kb-editor-open','editor.open','Open editor','Open the code editor for the current context','cmd','e','','editor','open','global','cmd','e','',''),
  ('kb-help-open','help.open','Keyboard help','Show all keyboard shortcuts','cmd','h','','help','open','global','cmd','h','',''),
  ('kb-context-copyname','context.copyName','Copy context name','Copy the current context key to the clipboard','cmd','c','n','context','copyName','global','cmd','c','n',''),
  ('kb-pr-open','pr.open','Open pull request','Open the pull request in the browser','cmd','g','p','pr','open','global','cmd','g','p',''),
  ('kb-pr-reload','pr.reload','Reload pull requests','Re-sync the pull-request list','cmd','p','r','pr','reload','global','cmd','p','r',''),
  ('kb-tabs-next','tabs.next','Next open context','Switch to the next open context tab','cmd',']','','tabs','next','global','cmd',']','',''),
  ('kb-tabs-prev','tabs.prev','Previous open context','Switch to the previous open context tab','cmd','[','','tabs','prev','global','cmd','[','',''),
  ('kb-editor-focus-up','editor.focusUp','Focus zone up','Move focus to the zone above','ctrl','arrowup','','editor','focusUp','editor','ctrl','arrowup','',''),
  ('kb-editor-focus-down','editor.focusDown','Focus zone down','Move focus to the zone below','ctrl','arrowdown','','editor','focusDown','editor','ctrl','arrowdown','',''),
  ('kb-editor-focus-left','editor.focusLeft','Focus zone left','Move focus to the zone to the left','ctrl','arrowleft','','editor','focusLeft','editor','ctrl','arrowleft','',''),
  ('kb-editor-focus-right','editor.focusRight','Focus zone right','Move focus to the zone to the right','ctrl','arrowright','','editor','focusRight','editor','ctrl','arrowright','',''),
  ('kb-sidebar-next','sidebar.next','Next sidebar tab','Switch FILES / GIT (forward)','cmd,shift',']','','sidebar','next','editor','cmd,shift',']','',''),
  ('kb-sidebar-prev','sidebar.prev','Previous sidebar tab','Switch FILES / GIT (back)','cmd,shift','[','','sidebar','prev','editor','cmd,shift','[','',''),
  ('kb-files-reload','files.reload','Reload files','Reload the file list from disk/git','cmd','r','','files','reload','files','cmd','r','',''),
  ('kb-files-focus-search','files.focusSearch','Focus file search','Move focus to the file filter input','cmd','f','f','files','focusSearch','files','cmd','f','f',''),
  ('kb-terminal-new','terminal.new','New terminal','Open a new terminal (same as +)','cmd','t','','terminal','new','terminal','cmd','t','',''),
  ('kb-terminal-here','terminal.here','Terminal here','Open a terminal at the selected path','cmd','t','h','terminal','here','terminal','cmd','t','h',''),
  ('kb-terminal-insert','terminal.insert','Insert command','Open the insert-command menu','cmd','t','i','terminal','insert','terminal','cmd','t','i',''),
  ('kb-terminal-next','terminal.next','Next terminal','Switch to the next terminal','cmd,shift',']','','terminal','next','terminal','cmd,shift',']','',''),
  ('kb-terminal-prev','terminal.prev','Previous terminal','Switch to the previous terminal','cmd,shift','[','','terminal','prev','terminal','cmd,shift','[','','')
ON CONFLICT(command) DO NOTHING;

-- +goose Down
DELETE FROM keybindings WHERE command IN (
  'search.global','editor.open','help.open','context.copyName','pr.open','pr.reload',
  'editor.focusUp','editor.focusDown','editor.focusLeft','editor.focusRight',
  'sidebar.next','sidebar.prev','files.reload','files.focusSearch','tabs.next','tabs.prev',
  'terminal.new','terminal.here','terminal.insert','terminal.next','terminal.prev');
ALTER TABLE keybindings DROP COLUMN default_sequence;
ALTER TABLE keybindings DROP COLUMN default_key_code;
ALTER TABLE keybindings DROP COLUMN default_modifier_keys;
ALTER TABLE keybindings DROP COLUMN sequence;
ALTER TABLE keybindings DROP COLUMN scope;
ALTER TABLE keybindings DROP COLUMN action;
ALTER TABLE keybindings DROP COLUMN object;
