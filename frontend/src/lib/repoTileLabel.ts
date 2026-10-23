// repoTileLabel compresses a repo + branch into a two-line tile label.
// Branches follow "feat_JH-5612_task_JH-5638_slug…" — the LAST ticket id is the
// specific task, which is the shortest thing that still tells tiles apart.
export function repoTileLabel(name: string, branch?: string): { top: string; sub: string } {
  const top = name.includes("/") ? name.split("/").pop()! : name;
  if (!branch) return { top, sub: "" };
  const tickets = branch.match(/[A-Z]+-\d+/g);
  return { top, sub: tickets?.length ? tickets[tickets.length - 1] : branch };
}
