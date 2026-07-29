import { useCallback, useEffect, useState } from "react";
import { ListContexts } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { effectiveStatus, type Context } from "../types";

export type ContextBadge = { unviewedCount: number; needsAttention: boolean };

/**
 * Per-context badge data for the sidebar plates: count of new items and whether
 * the context needs attention. Refetches on the background-refresh event and
 * whenever the active tab changes (opening a context marks its items viewed).
 */
export function useContextBadges(activeTabId: string | null): Map<string, ContextBadge> {
  const [badges, setBadges] = useState<Map<string, ContextBadge>>(new Map());

  const load = useCallback(async () => {
    try {
      const list = (await ListContexts()) as Context[] | null;
      const next = new Map<string, ContextBadge>();
      (list ?? []).forEach((c) => {
        next.set(c.id, {
          unviewedCount: c.unviewedCount ?? 0,
          needsAttention: effectiveStatus(c) === "needs_attention",
        });
      });
      setBadges(next);
    } catch {
      /* transient; keep last known badges */
    }
  }, []);

  // Reload on mount and whenever the active tab changes.
  useEffect(() => { void load(); }, [load, activeTabId]);

  // Reload when the background refresh reports context changes.
  useEffect(() => {
    const off = EventsOn("bg-refresh:contexts", () => { void load(); });
    return () => { off(); };
  }, [load]);

  return badges;
}
