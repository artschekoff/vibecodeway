-- +goose Up

CREATE TABLE IF NOT EXISTS keybindings (
    id            TEXT PRIMARY KEY,
    command       TEXT NOT NULL UNIQUE,
    label         TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    modifier_keys TEXT NOT NULL,
    key_code      TEXT NOT NULL,
    updated_at    TEXT NOT NULL DEFAULT ''
);

INSERT OR IGNORE INTO keybindings (id, command, label, description, modifier_keys, key_code, updated_at)
VALUES
  ('kb-quick-attach', 'quick_attach', 'Quick Attach',
   'Select text in any app and press this shortcut to open the context picker. Works globally.',
   'cmd,shift', 'c', ''),
  ('kb-send-chat', 'send_chat', 'Send message in chat',
   'In the context chat window — sends the message without inserting a new line.',
   'cmd', 'enter', '');

-- +goose Down
DROP TABLE IF EXISTS keybindings;
