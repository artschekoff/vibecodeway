export type Scope = "global" | "editor" | "terminal" | "files" | "git";
export const SCOPE_PRIORITY: Scope[] = ["terminal", "files", "git", "editor", "global"];
