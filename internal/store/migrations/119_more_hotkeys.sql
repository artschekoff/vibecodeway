-- +goose Up
-- Four new rebindable commands (registry ids must match frontend/src/hotkeys/registry.ts):
-- board.open ⌘⇧M, ai.ask ⌘⇧A, context.refresh ⌘⇧R (global), tour.start ⌘⇧U (editor).
INSERT INTO keybindings
  (id, command, label, description, modifier_keys, key_code, sequence, object, action, scope,
   default_modifier_keys, default_key_code, default_sequence, updated_at)
VALUES
  ('kb-board-open','board.open','Open My Board','Open the My Board tab','cmd,shift','m','','board','open','global','cmd,shift','m','',''),
  ('kb-ai-ask','ai.ask','Open Ask AI','Open the Ask AI tab','cmd,shift','a','','ai','ask','global','cmd,shift','a','',''),
  ('kb-context-refresh','context.refresh','Refresh context','Refresh the active context from external sources','cmd,shift','r','','context','refresh','global','cmd,shift','r','',''),
  ('kb-tour-start','tour.start','Start code tour','Start (or resume) the code tour in the editor','cmd,shift','u','','tour','start','editor','cmd,shift','u','','');

-- +goose Down
DELETE FROM keybindings WHERE id IN ('kb-board-open','kb-ai-ask','kb-context-refresh','kb-tour-start');
