-- +goose Up
-- Context-detail topbar hotkeys (registry ids must match frontend/src/hotkeys/registry.ts):
-- back ⌘⇧B, mark-all-viewed ⌘⇧V, assign-authors ⌘⇧P, edit ⌘⇧E, solve-with-AI ⌘⇧S.
INSERT INTO keybindings
  (id, command, label, description, modifier_keys, key_code, sequence, object, action, scope,
   default_modifier_keys, default_key_code, default_sequence, updated_at)
VALUES
  ('kb-context-back','context.back','Back','Go back from the context detail view','cmd,shift','b','','context','back','global','cmd,shift','b','',''),
  ('kb-context-mark-all-viewed','context.markAllViewed','Mark all viewed','Mark every item in the context as viewed','cmd,shift','v','','context','markAllViewed','global','cmd,shift','v','',''),
  ('kb-context-assign-authors','context.assignAuthors','Assign authors','Open the unknown-authors assignment dialog','cmd,shift','p','','context','assignAuthors','global','cmd,shift','p','',''),
  ('kb-context-edit','context.edit','Edit context','Open the Edit Context form','cmd,shift','e','','context','edit','global','cmd,shift','e','',''),
  ('kb-context-solve-ai','context.solveAI','Solve with AI','Open the Solve with AI panel','cmd,shift','s','','context','solveAI','global','cmd,shift','s','','');

-- +goose Down
DELETE FROM keybindings WHERE id IN ('kb-context-back','kb-context-mark-all-viewed','kb-context-assign-authors','kb-context-edit','kb-context-solve-ai');
