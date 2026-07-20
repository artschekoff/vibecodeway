import { useEffect, useState } from "react";
import { GetActivityFeed } from "../../wailsjs/go/main/App";
import { useTabContext } from "../contexts/TabContext";
import type { ActivityFeedItem } from "../types";
import { activitySource, describeActivity } from "../lib/activityFeed";

export default function ActivityFeedPage() {
  const { openContextTab } = useTabContext();
  const [items, setItems] = useState<ActivityFeedItem[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    void GetActivityFeed(100)
      .then((res) => { if (!cancelled) setItems(res ?? []); })
      .catch(() => { if (!cancelled) setItems([]); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, []);

  return (
    <div className="page">
      <div className="topbar">
        <span className="topbar-title">Activity Feed</span>
      </div>
      <div className="content" style={{ maxWidth: 720 }}>
        {loading && <p style={{ color: "var(--text-muted)" }}>Loading…</p>}
        {!loading && items.length === 0 && (
          <p style={{ color: "var(--text-muted)" }}>No activity yet.</p>
        )}
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {items.map((item) => {
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
                <span data-testid="activity-desc" style={{ flex: 1 }}>
                  {describeActivity(item)}
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
