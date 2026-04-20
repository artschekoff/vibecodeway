import { useEffect, useState } from "react";
import ReactMarkdown from "react-markdown";
import {
  GetConsolidatedContextLog,
  UpdateConsolidatedContextLog,
  InitializeConsolidatedLog,
} from "../../wailsjs/go/main/App";
import type { ConsolidatedLog } from "../types";

interface Props {
  contextId: string;
  reloadKey?: number;
}

export default function ConsolidatedLogView({ contextId, reloadKey }: Props) {
  const [log, setLog] = useState<ConsolidatedLog | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [initializing, setInitializing] = useState(false);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");

  const reload = async () => {
    setLoading(true);
    try {
      const c = await GetConsolidatedContextLog(contextId);
      setLog(c);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [contextId, reloadKey]);

  const handleInitialize = async () => {
    setInitializing(true);
    setError("");
    try {
      const created = await InitializeConsolidatedLog(contextId);
      setLog(created);
    } catch (e) {
      setError(String(e));
    } finally {
      setInitializing(false);
    }
  };

  const handleStartEdit = () => {
    setDraft(log?.contentMd ?? "");
    setEditing(true);
  };

  const handleSave = async () => {
    try {
      const updated = await UpdateConsolidatedContextLog(contextId, draft);
      setLog(updated);
      setEditing(false);
    } catch (e) {
      setError(String(e));
    }
  };

  if (loading) return <div className="loading">Loading…</div>;

  const empty = !log || !log.contentMd;

  return (
    <div className="consolidated-panel">
      {error && <div className="error-banner">{error}</div>}

      <div className="consolidated-toolbar">
        {empty ? (
          <button
            className="btn-primary"
            onClick={() => void handleInitialize()}
            disabled={initializing}
          >
            {initializing ? "Initializing…" : "Initialize log"}
          </button>
        ) : editing ? (
          <>
            <button className="btn-primary" onClick={() => void handleSave()}>
              Save
            </button>
            <button className="btn-ghost" onClick={() => setEditing(false)}>
              Cancel
            </button>
          </>
        ) : (
          <>
            <button className="btn-ghost" onClick={handleStartEdit}>
              ✎ Edit
            </button>
            <span style={{ flex: 1 }} />
            {log && (
              <span style={{ fontSize: 11, color: "var(--text-muted)" }}>
                v{log.version} · {log.updatedBy} · {formatDate(log.updatedAt)}
              </span>
            )}
          </>
        )}
      </div>

      {empty && !initializing && (
        <div className="chat-empty">
          No consolidated log yet. Initialize it — the AI will analyze all current context
          content and write a starter log.
        </div>
      )}

      {!empty && !editing && (
        <div className="consolidated-body">
          <ReactMarkdown>{log!.contentMd}</ReactMarkdown>
        </div>
      )}

      {editing && (
        <textarea
          className="chat-input"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          rows={24}
        />
      )}
    </div>
  );
}

function formatDate(s: string): string {
  try {
    return new Date(s).toLocaleString();
  } catch {
    return s;
  }
}
