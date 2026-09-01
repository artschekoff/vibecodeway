import { useEffect, useState, useCallback, useRef, useMemo } from "react";
import { RefreshCw } from "lucide-react";
import { GetActivityFeed, SearchContextIDs, ListMyAssignedJiraIssues, MarkMyAssignedJiraIssuesSeen } from "../../wailsjs/go/main/App";
import { useTabContext } from "../contexts/TabContext";
import type { ActivityFeedItem } from "../types";
import { activitySource, describeActivity } from "../lib/activityFeed";
import type { store } from "../../wailsjs/go/models";

export default function ActivityFeedPage() {
  const { openContextTab } = useTabContext();
  const [items, setItems] = useState<ActivityFeedItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [matchedIDs, setMatchedIDs] = useState<Set<string> | null>(null);
  const [assignedJira, setAssignedJira] = useState<store.BoardJiraAssignment[]>([]);
  const searchTimer = useRef<ReturnType<typeof setTimeout>>();

  const load = useCallback(() => {
    setLoading(true);
    void GetActivityFeed(100)
      .then((res) => setItems((res ?? []) as unknown as ActivityFeedItem[]))
      .catch(() => setItems([]))
      .finally(() => setLoading(false));
    void ListMyAssignedJiraIssues("priority")
      .then((rows) => {
        const unseen = (rows ?? []).filter((r) => !r.seen);
        setAssignedJira(unseen);
        if (unseen.length > 0) void MarkMyAssignedJiraIssuesSeen();
      })
      .catch(() => setAssignedJira([]));
  }, []);

  useEffect(() => { load(); }, [load]);

  // ── Search ── same logic as the Contexts list: debounced backend
  // full-text search returning matching context IDs; the feed is then
  // filtered to events belonging to those contexts.
  const handleSearchChange = useCallback((value: string) => {
    setSearchQuery(value);
    clearTimeout(searchTimer.current);
    if (!value.trim()) { setMatchedIDs(null); return; }
    searchTimer.current = setTimeout(async () => {
      try {
        const ids = await SearchContextIDs(value.trim());
        setMatchedIDs(new Set(ids ?? []));
      } catch {
        setMatchedIDs(null);
      }
    }, 200);
  }, []);

  const filtered = useMemo(
    () => (matchedIDs ? items.filter((i) => matchedIDs.has(i.contextId)) : items),
    [items, matchedIDs],
  );

  return (
    <div className="page">
      <div className="topbar">
        <span className="topbar-title">Activity Feed</span>
        <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 8 }}>
          <div className="context-search">
            <input
              type="text"
              className="context-search-input"
              placeholder="Search…"
              value={searchQuery}
              onChange={(e) => handleSearchChange(e.target.value)}
              autoCorrect="off"
              autoCapitalize="none"
              spellCheck={false}
            />
            {searchQuery && (
              <button className="context-search-clear" onClick={() => handleSearchChange("")} title="Clear search">×</button>
            )}
          </div>
          <button className="btn-secondary" onClick={() => load()} disabled={loading} title="Refresh">
            <RefreshCw size={14} /> Refresh
          </button>
        </div>
      </div>
      <div className="content" style={{ maxWidth: 720 }}>
        {loading && <p style={{ color: "var(--text-muted)" }}>Loading…</p>}
        {!loading && filtered.length === 0 && (
          <p style={{ color: "var(--text-muted)" }}>
            {searchQuery ? "No matching activity." : "No activity yet."}
          </p>
        )}
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {assignedJira.map((it) => (
            <div key={`jira-${it.key}`} className="activity-item" style={{ display: "flex", gap: 8, alignItems: "center", padding: "6px 8px" }}>
              <span>Jira</span>
              <span data-testid="assigned-jira-desc" style={{ flex: 1 }}>{it.summary}</span>
              <span style={{ fontWeight: 600 }}>{it.key}</span>
              <span className="board-badge-new" style={{ fontSize: 10, color: "var(--accent)" }}>new</span>
            </div>
          ))}
          {filtered.map((item) => {
            const source = activitySource(item);
            return (
              <button
                key={item.id}
                onClick={() => openContextTab(item.contextId, item.contextKey)}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 10,
                  padding: "10px 12px",
                  textAlign: "left",
                  color: "var(--text)",
                  background: "var(--bg-elevated, rgba(255,255,255,0.03))",
                  border: "1px solid var(--border, rgba(255,255,255,0.08))",
                  borderRadius: 8,
                  cursor: "pointer",
                }}
              >
                <span
                  style={{
                    fontSize: 11,
                    fontWeight: 600,
                    padding: "2px 6px",
                    borderRadius: 4,
                    color: source === "jira" ? "#4c9aff" : "#c9a4ff",
                    border: "1px solid currentColor",
                  }}
                >
                  {source === "jira" ? "Jira" : "GitHub"}
                </span>
                <span style={{ flex: 1, display: "flex", flexDirection: "column", gap: 2, minWidth: 0 }}>
                  <span data-testid="activity-desc">{describeActivity(item)}</span>
                  {item.note && (
                    <span
                      data-testid="activity-note"
                      style={{ fontSize: 11, color: "var(--text-muted)", overflow: "hidden", textOverflow: "ellipsis" }}
                      title={item.note}
                    >
                      {item.note}
                    </span>
                  )}
                </span>
                <span style={{ fontSize: 12, color: "var(--text-muted)" }}>{item.contextKey}</span>
                <span style={{ fontSize: 11, color: "var(--text-muted)" }}>
                  {new Date(item.createdAt).toLocaleString()}
                </span>
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}
