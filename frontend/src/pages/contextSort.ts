import { effectiveStatus, type Context } from "../types";

// Columns of the Contexts table that sort on header click. null sort = the
// user's manual drag order.
export type ContextSortCol =
  | "active" | "key" | "status" | "issues" | "importance" | "unread" | "updated" | "latest";

const keyOf: Record<ContextSortCol, (c: Context) => string | number> = {
  active: (c) => (c.status === "active" ? 1 : 0),
  key: (c) => c.key.toLowerCase(),
  status: (c) => effectiveStatus(c),
  issues: (c) => c.issueCount ?? 0,
  importance: (c) => c.importance ?? -1,
  unread: (c) => c.unviewedCount ?? 0,
  updated: (c) => c.updatedAt ?? "",   // ISO timestamps: lexicographic = chronological
  latest: (c) => c.lastItemAt ?? "",
};

// sortContexts returns a NEW array sorted by col/dir; stable for equal keys.
export function sortContexts(list: Context[], col: ContextSortCol, dir: "asc" | "desc"): Context[] {
  const get = keyOf[col];
  const sign = dir === "asc" ? 1 : -1;
  return [...list].sort((a, b) => {
    const ka = get(a);
    const kb = get(b);
    if (ka < kb) return -sign;
    if (ka > kb) return sign;
    return 0;
  });
}
