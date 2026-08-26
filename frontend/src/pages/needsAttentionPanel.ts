import { effectiveStatus, type Context } from "../types";

// needsAttentionTiles picks the contexts the Needs Attention panel should show.
//
// The panel used to filter on the snapshot status alone, but ListContextsByOrg
// returns archived contexts too — so contexts the user had already closed out
// kept taking up space. An archived context is nobody's move, whoever set the
// flag and whatever the reason was.
export function needsAttentionTiles(contexts: Context[]): Context[] {
  return contexts.filter((c) => c.status === "active" && effectiveStatus(c) === "needs_attention");
}
