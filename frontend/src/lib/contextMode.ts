// PR-review contexts are keyed "owner/name#prNumber" (see ensurePRReviewContext
// on the backend); every other context uses its own key (e.g. a Jira key).
const PR_REVIEW_KEY = /^[^/\s]+\/[^/\s]+#\d+$/;

export type EditorMode = "review" | "context";

/** Infer whether an editor tab is reviewing a PR or editing a context. */
export function editorModeForKey(contextKey?: string): EditorMode {
  return contextKey && PR_REVIEW_KEY.test(contextKey) ? "review" : "context";
}

/** Human-readable badge label for an editor mode. */
export function editorModeLabel(mode: EditorMode): string {
  return mode === "review" ? "PR Review" : "Context Editing";
}
