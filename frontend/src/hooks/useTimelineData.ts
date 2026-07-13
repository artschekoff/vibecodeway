import { useCallback, useEffect, useState } from "react";
import {
  ListContextsByOrg,
  ListEvents,
  ListWorkflowStatuses,
  GetContextRelations,
} from "../../wailsjs/go/main/App";
import { toLaneEvent, type Lane, type RelationEdge } from "../lib/timeline";

export interface WorkflowStatusInfo {
  slug: string;
  label: string;
  color: string;
}

export interface TimelineData {
  lanes: Lane[];
  relations: RelationEdge[];
  statuses: Record<string, WorkflowStatusInfo>;
  loading: boolean;
  error: string | null;
  reload: () => void;
}

export function useTimelineData(orgId: string): TimelineData {
  const [lanes, setLanes] = useState<Lane[]>([]);
  const [relations, setRelations] = useState<RelationEdge[]>([]);
  const [statuses, setStatuses] = useState<Record<string, WorkflowStatusInfo>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [ctxs, wfStatuses] = await Promise.all([
        ListContextsByOrg(orgId),
        ListWorkflowStatuses(),
      ]);
      const active = (ctxs ?? []).filter((c) => c.status === "active");

      const statusMap: Record<string, WorkflowStatusInfo> = {};
      for (const s of wfStatuses ?? []) {
        statusMap[s.slug] = { slug: s.slug, label: s.label, color: s.color };
      }

      const perContext = await Promise.all(
        active.map(async (c) => {
          const [evts, rels] = await Promise.all([
            ListEvents(c.id).catch(() => []),
            GetContextRelations(c.id).catch(() => []),
          ]);
          const lane: Lane = {
            contextId: c.id,
            contextKey: c.key,
            status: c.snapshotStatus || "",
            events: (evts ?? [])
              .map((e) => toLaneEvent(e))
              .sort((a, b) => a.timestamp.localeCompare(b.timestamp)),
          };
          const edges: RelationEdge[] = (rels ?? []).map((r) => ({
            id: r.id,
            sourceContextId: r.contextId,
            targetContextId: r.targetId,
            targetKey: r.targetKey,
            relationType: r.relationType,
          }));
          return { lane, edges };
        }),
      );

      const visible = new Set(active.map((c) => c.id));
      setLanes(perContext.map((p) => p.lane));
      setRelations(
        perContext.flatMap((p) => p.edges).filter((e) => visible.has(e.targetContextId)),
      );
      setStatuses(statusMap);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [orgId]);

  useEffect(() => {
    void load();
  }, [load]);

  return { lanes, relations, statuses, loading, error, reload: load };
}
