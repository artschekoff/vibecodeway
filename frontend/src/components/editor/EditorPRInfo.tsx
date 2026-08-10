import { useCallback, useEffect, useState } from "react";
import { ExternalLink, GitBranch, MessageSquare } from "lucide-react";
import { GetVCSPRsForContext, UpdatePRBranch } from "../../../wailsjs/go/main/App";
import { BrowserOpenURL } from "../../../wailsjs/runtime/runtime";
import type { PullRequest } from "../../types";
import { PRStatusBadge } from "../PRStatusBadge";
import { countUnreadComments } from "../../lib/prComments";

/**
 * Rich PR info for the editor sidebar: the single PR in PR-review mode, or every
 * PR linked to the context in context-editing mode. Renders nothing when there
 * are no linked PRs.
 */
export function EditorPRInfo({ contextId, activeRepoId, refreshSignal }: { contextId: string; activeRepoId?: string; refreshSignal?: number }) {
  const [prs, setPrs] = useState<PullRequest[]>([]);
  const [updating, setUpdating] = useState<Set<string>>(new Set());

  const load = useCallback(() => {
    return GetVCSPRsForContext(contextId)
      .then((rows) => setPrs((rows as PullRequest[]) ?? []))
      .catch(() => setPrs([]));
  }, [contextId]);

  useEffect(() => {
    let active = true;
    void GetVCSPRsForContext(contextId)
      .then((rows) => { if (active) setPrs((rows as PullRequest[]) ?? []); })
      .catch(() => { if (active) setPrs([]); });
    return () => { active = false; };
  }, [contextId, refreshSignal]);

  const onUpdateBranch = async (pr: PullRequest) => {
    setUpdating((s) => new Set(s).add(pr.id));
    try {
      await UpdatePRBranch(pr.repoId, pr.prNumber);
      await load();
    } catch {
      // best-effort: GitHub rejects if not actually behind; leave the button.
    } finally {
      setUpdating((s) => { const n = new Set(s); n.delete(pr.id); return n; });
    }
  };

  // Follow the editor's repo selector: show only the selected repo's PR(s).
  // Unset => show every linked PR (backward compatible with single-repo contexts).
  const shown = activeRepoId ? prs.filter((pr) => pr.repoId === activeRepoId) : prs;

  if (shown.length === 0) return null;

  return (
    <div style={{ padding: "8px 12px", borderBottom: "1px solid var(--border)", display: "flex", flexDirection: "column", gap: 10 }}>
      {shown.map((pr) => {
        const unread = countUnreadComments(pr.comments);
        const unresolved = (pr.comments ?? []).filter((c) => !c.isResolved).length;
        return (
          <div key={pr.id} style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <button
              onClick={() => pr.url && BrowserOpenURL(pr.url)}
              title={pr.url || pr.title}
              style={{ display: "flex", alignItems: "center", gap: 6, background: "none", border: "none", padding: 0, cursor: "pointer", color: "var(--text)", textAlign: "left", minWidth: 0 }}
            >
              <span style={{ fontSize: 12, fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                {pr.title || `#${pr.prNumber}`}
              </span>
              <ExternalLink size={12} style={{ flexShrink: 0, color: "var(--text-muted)" }} />
            </button>
            <div style={{ fontSize: 11, color: "var(--text-muted)", display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
              <span>#{pr.prNumber}</span>
              {pr.author && <span>by {pr.author}</span>}
              {pr.headBranch && (
                <span style={{ display: "inline-flex", alignItems: "center", gap: 3 }}>
                  <GitBranch size={11} /> {pr.headBranch}
                </span>
              )}
              {unread > 0 && (
                <span style={{ display: "inline-flex", alignItems: "center", gap: 3 }} title={`${unread} unread comment(s)`}>
                  <MessageSquare size={11} /> {unread}
                </span>
              )}
              {unresolved > 0 && (
                <span style={{ display: "inline-flex", alignItems: "center", gap: 3, color: "var(--danger, #e5534b)" }} title={`${unresolved} unresolved comment(s)`}>
                  ⚠ {unresolved}
                </span>
              )}
            </div>
            <PRStatusBadge status={pr.state} reviewStatus={pr.reviewStatus} mergeable={pr.mergeable} ciStatus={pr.ciStatus} isDraft={pr.isDraft} />
            {pr.state === "open" && pr.mergeStateStatus === "BEHIND" && (
              <button
                onClick={() => void onUpdateBranch(pr)}
                disabled={updating.has(pr.id)}
                style={{ alignSelf: "flex-start", fontSize: 11, padding: "2px 8px", borderRadius: 4, border: "1px solid var(--border)", background: "var(--surface)", color: "var(--text)", cursor: updating.has(pr.id) ? "default" : "pointer" }}
              >
                {updating.has(pr.id) ? "Updating…" : "Update branch"}
              </button>
            )}
          </div>
        );
      })}
    </div>
  );
}
