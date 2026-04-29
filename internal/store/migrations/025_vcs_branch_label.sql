-- 025_vcs_branch_label.sql
ALTER TABLE context_vcs_branches ADD COLUMN label TEXT NOT NULL DEFAULT '';
