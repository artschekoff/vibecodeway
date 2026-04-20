import { useEffect, useState } from "react";
import ReactMarkdown from "react-markdown";
import {
  ListContextLogs,
  CreateContextLog,
  UpdateContextLog,
  DeleteContextLog,
  AnalyzeContextChanges,
} from "../../wailsjs/go/main/App";
import type { ContextLog } from "../types";

interface Props {
  contextId: string;
  onChanged?: () => void;
}

export default function ContextLogList({ contextId, onChanged }: Props) {
  const [logs, setLogs] = useState<ContextLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [analyzing, setAnalyzing] = useState(false);

  const [editingId, setEditingId] = useState<string | null>(null);
  const [editDraft, setEditDraft] = useState("");

  const [adding, setAdding] = useState(false);
  const [addDraft, setAddDraft] = useState("");

  const reload = async () => {
    setLoading(true);
    try {
      const list = await ListContextLogs(contextId);
      setLogs(list ?? []);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [contextId]);

  const handleAnalyze = async () => {
    setAnalyzing(true);
    setError("");
    try {
      await AnalyzeContextChanges(contextId);
      await reload();
      onChanged?.();
    } catch (e) {
      setError(String(e));
    } finally {
      setAnalyzing(false);
    }
  };

  const handleStartEdit = (log: ContextLog) => {
    setEditingId(log.id);
    setEditDraft(log.summaryMd);
  };

  const handleCancelEdit = () => {
    setEditingId(null);
    setEditDraft("");
  };

  const handleSaveEdit = async () => {
    if (!editingId) return;
    try {
      await UpdateContextLog(editingId, editDraft);
      setEditingId(null);
      setEditDraft("");
      await reload();
      onChanged?.();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleDelete = async (id: string) => {
    if (!window.confirm("Delete this context log entry? This action cannot be undone.")) return;
    try {
      await DeleteContextLog(id);
      await reload();
      onChanged?.();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleAdd = async () => {
    const text = addDraft.trim();
    if (!text) return;
    try {
      await CreateContextLog(contextId, text);
      setAdding(false);
      setAddDraft("");
      await reload();
      onChanged?.();
    } catch (e) {
      setError(String(e));
    }
  };

  return (
    <div className="log-panel">
      {error && <div className="error-banner">{error}</div>}

      <div className="log-panel-toolbar">
        <button
          className="btn-primary"
          onClick={() => void handleAnalyze()}
          disabled={analyzing}
          title="Ask AI to analyze the current context state"
        >
          {analyzing ? "Analyzing…" : "Analyze"}
        </button>
        <button className="btn-ghost" onClick={() => setAdding((v) => !v)}>
          {adding ? "Cancel" : "+ Manual entry"}
        </button>
      </div>

      {adding && (
        <div className="log-entry log-entry--editing">
          <textarea
            className="chat-input"
            placeholder="Markdown description of the change…"
            value={addDraft}
            onChange={(e) => setAddDraft(e.target.value)}
            rows={6}
            autoFocus
          />
          <div className="log-entry-actions">
            <button className="btn-primary" onClick={() => void handleAdd()} disabled={!addDraft.trim()}>
              Save
            </button>
            <button className="btn-ghost" onClick={() => { setAdding(false); setAddDraft(""); }}>
              Cancel
            </button>
          </div>
        </div>
      )}

      {loading && <div className="loading">Loading…</div>}

      {!loading && logs.length === 0 && !adding && (
        <div className="chat-empty">
          No entries yet. Run Refresh or click "Analyze".
        </div>
      )}

      {logs.map((log) => {
        const isEditing = editingId === log.id;
        return (
          <div key={log.id} className="log-entry">
            <div className="log-entry-header">
              <span className="log-entry-when">{formatDate(log.createdAt)}</span>
              <span className={`log-entry-trigger log-entry-trigger--${log.trigger}`}>{log.trigger}</span>
              {log.isManual && <span className="log-entry-tag">manual</span>}
              {log.editedAt && <span className="log-entry-tag">edited</span>}
              {log.model && <span className="log-entry-tag">{log.model}</span>}
              <span style={{ flex: 1 }} />
              {!isEditing && (
                <>
                  <button className="btn-ghost" onClick={() => handleStartEdit(log)} title="Edit">
                    ✎
                  </button>
                  <button
                    className="btn-ghost"
                    style={{ color: "var(--danger)" }}
                    onClick={() => void handleDelete(log.id)}
                    title="Delete"
                  >
                    ✕
                  </button>
                </>
              )}
            </div>
            {isEditing ? (
              <>
                <textarea
                  className="chat-input"
                  value={editDraft}
                  onChange={(e) => setEditDraft(e.target.value)}
                  rows={8}
                  autoFocus
                />
                <div className="log-entry-actions">
                  <button className="btn-primary" onClick={() => void handleSaveEdit()}>
                    Save
                  </button>
                  <button className="btn-ghost" onClick={handleCancelEdit}>
                    Cancel
                  </button>
                </div>
              </>
            ) : (
              <div className="log-entry-body">
                <ReactMarkdown>{log.summaryMd || "_(empty)_"}</ReactMarkdown>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

function formatDate(s: string): string {
  try {
    const d = new Date(s);
    return d.toLocaleString();
  } catch {
    return s;
  }
}
