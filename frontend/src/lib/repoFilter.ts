// Row-level filter predicates for the Repositories tab. The query `q` is assumed to be
// already trimmed and lowercased by the caller. An empty query matches everything.

/** Matches a workspace by its branch name or the basename of its worktree path. */
export function workspaceMatchesQuery(w: { branch: string; worktreePath: string }, q: string): boolean {
  if (q === "") return true;
  const folder = w.worktreePath.split("/").pop() ?? "";
  return w.branch.toLowerCase().includes(q) || folder.toLowerCase().includes(q);
}

/** Matches a local branch by its name. */
export function branchMatchesQuery(b: { name: string }, q: string): boolean {
  if (q === "") return true;
  return b.name.toLowerCase().includes(q);
}
