import { useEffect, useRef, useState } from "react";
import { Pencil, Trash2, Check, X, Plus } from "lucide-react";
import {
  ListWorkflowStatuses,
  CreateWorkflowStatus,
  UpdateWorkflowStatus,
  DeleteWorkflowStatus,
} from "../../wailsjs/go/main/App";
import type { WorkflowStatus } from "../types";
import { ProseEditor } from "./ProseEditor";

const SLUG_RE = /^[a-z][a-z0-9_]*$/;

function deriveSlug(label: string): string {
  return label
    .toLowerCase()
    .replace(/\s+/g, "_")
    .replace(/[^a-z0-9_]/g, "")
    .replace(/^_+|_+$/g, "");
}

export default function WorkflowStatusManager() {
  const [statuses, setStatuses] = useState<WorkflowStatus[]>([]);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editLabel, setEditLabel] = useState("");
  const [editAiHint, setEditAiHint] = useState("");
  const [editColor, setEditColor] = useState("");
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const confirmTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [adding, setAdding] = useState(false);
  const [newLabel, setNewLabel] = useState("");
  const [newSlug, setNewSlug] = useState("");
  const [newAiHint, setNewAiHint] = useState("");
  const [newColor, setNewColor] = useState("");
  const [slugManual, setSlugManual] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    ListWorkflowStatuses().then(setStatuses).catch(console.error);
  }, []);

  async function saveEdit(id: string) {
    try {
      const updated = await UpdateWorkflowStatus(id, editLabel.trim(), editAiHint.trim(), editColor);
      setStatuses((prev) => prev.map((s) => (s.id === id ? updated : s)));
      setEditingId(null);
      setError("");
    } catch (e) {
      setError(String(e));
    }
  }

  function requestDelete(id: string) {
    if (confirmTimerRef.current) clearTimeout(confirmTimerRef.current);
    setConfirmDeleteId(id);
    confirmTimerRef.current = setTimeout(() => setConfirmDeleteId(null), 3000);
  }

  async function handleDelete(id: string) {
    setConfirmDeleteId(null);
    if (confirmTimerRef.current) clearTimeout(confirmTimerRef.current);
    try {
      await DeleteWorkflowStatus(id);
      setStatuses((prev) => prev.filter((s) => s.id !== id));
      setError("");
    } catch (e) {
      setError(String(e));
    }
  }

  async function handleAdd() {
    const slug = newSlug.trim();
    const label = newLabel.trim();
    if (!label) { setError("Label is required."); return; }
    if (!SLUG_RE.test(slug)) {
      setError("Slug must start with a letter and contain only lowercase letters, digits, or underscores.");
      return;
    }
    try {
      const created = await CreateWorkflowStatus(slug, label, newAiHint.trim(), newColor);
      setStatuses((prev) => [...prev, created]);
      setAdding(false);
      setNewLabel("");
      setNewSlug("");
      setNewAiHint("");
      setNewColor("");
      setSlugManual(false);
      setError("");
    } catch (e) {
      setError(String(e));
    }
  }

  return (
    <div className="workflow-status-manager">
      <h3 className="settings-section-title">Workflow statuses</h3>
      {error && <p className="settings-error">{error}</p>}

      <ul className="workflow-status-list">
        {statuses.map((st) => (
          <li key={st.id} className="workflow-status-item">
            {editingId === st.id ? (
              <>
                <div style={{ flex: 1, display: "flex", flexDirection: "column", gap: 4 }}>
                  <input
                    className="workflow-status-input"
                    value={editLabel}
                    onChange={(e) => setEditLabel(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") saveEdit(st.id);
                      if (e.key === "Escape") setEditingId(null);
                    }}
                    autoFocus
                  />
                  <label style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 11, color: "var(--text-muted)" }}>
                    <input type="color" value={editColor || "#6366f1"} onChange={(e) => setEditColor(e.target.value)} style={{ width: 24, height: 20, padding: 0, border: "none", cursor: "pointer", background: "none" }} />
                    Color
                    {editColor && <button type="button" style={{ fontSize: 10, background: "none", border: "none", color: "var(--text-muted)", cursor: "pointer", padding: 0 }} onClick={() => setEditColor("")}>clear</button>}
                  </label>
                  <ProseEditor
                    className="form-control"
                    style={{ resize: "vertical", minHeight: 56 }}
                    placeholder="AI hint: when to use this status and what signals a transition…"
                    value={editAiHint}
                    onChange={(e) => setEditAiHint(e.target.value)}
                  />
                </div>
                <button className="icon-btn" onClick={() => saveEdit(st.id)} title="Save">
                  <Check size={14} />
                </button>
                <button className="icon-btn" onClick={() => setEditingId(null)} title="Cancel">
                  <X size={14} />
                </button>
              </>
            ) : (
              <>
                <span className="workflow-status-label">{st.label}</span>
                {st.aiHint && (
                  <details style={{ marginTop: 3 }}>
                    <summary style={{ fontSize: 11, cursor: "pointer", color: "var(--text-muted)" }}>AI hint</summary>
                    <p style={{ fontSize: 11, margin: "3px 0 0", color: "var(--text-muted)", lineHeight: 1.4 }}>{st.aiHint}</p>
                  </details>
                )}
                <span className="workflow-status-slug">{st.slug}</span>
                {st.isSystem && <span className="workflow-status-system-badge">system</span>}
                <div className="workflow-status-actions">
                  <button
                    className="icon-btn"
                    onClick={() => { setEditingId(st.id); setEditLabel(st.label); setEditAiHint(st.aiHint ?? ""); setEditColor(st.color ?? ""); setError(""); }}
                    title="Edit label"
                  >
                    <Pencil size={14} />
                  </button>
                  {!st.isSystem && (
                    <button
                      className={confirmDeleteId === st.id ? "icon-btn icon-btn--danger" : "icon-btn"}
                      onClick={() => {
                        if (confirmDeleteId === st.id) void handleDelete(st.id);
                        else requestDelete(st.id);
                      }}
                      title={confirmDeleteId === st.id ? "Confirm delete" : "Delete"}
                      style={confirmDeleteId === st.id ? { fontSize: 10, width: "auto", padding: "2px 6px" } : undefined}
                    >
                      {confirmDeleteId === st.id ? "Confirm" : <Trash2 size={14} />}
                    </button>
                  )}
                </div>
              </>
            )}
          </li>
        ))}
      </ul>

      {adding ? (
        <div className="workflow-status-add-form">
          <div className="workflow-status-add-fields">
            <input
              className="workflow-status-input"
              placeholder="Label (e.g. Blocked)"
              value={newLabel}
              onChange={(e) => {
                setNewLabel(e.target.value);
                if (!slugManual) setNewSlug(deriveSlug(e.target.value));
              }}
              autoFocus
            />
            <input
              className="workflow-status-input workflow-status-input--slug"
              placeholder="Slug (e.g. blocked)"
              value={newSlug}
              onChange={(e) => { setNewSlug(e.target.value); setSlugManual(true); }}
            />
            <label style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 11, color: "var(--text-muted)" }}>
              <input type="color" value={newColor || "#6366f1"} onChange={(e) => setNewColor(e.target.value)} style={{ width: 24, height: 20, padding: 0, border: "none", cursor: "pointer", background: "none" }} />
              Color (optional)
              {newColor && <button type="button" style={{ fontSize: 10, background: "none", border: "none", color: "var(--text-muted)", cursor: "pointer", padding: 0 }} onClick={() => setNewColor("")}>clear</button>}
            </label>
            <ProseEditor
              className="form-control"
              style={{ resize: "vertical", minHeight: 56 }}
              placeholder="AI hint (optional): when to use this status and what signals a transition…"
              value={newAiHint}
              onChange={(e) => setNewAiHint(e.target.value)}
            />
          </div>
          <div className="workflow-status-add-actions">
            <button className="btn btn--primary" onClick={handleAdd}>Add</button>
            <button
              className="btn btn--ghost"
              onClick={() => {
                setAdding(false);
                setNewLabel("");
                setNewSlug("");
                setNewAiHint("");
                setNewColor("");
                setSlugManual(false);
                setError("");
              }}
            >
              Cancel
            </button>
          </div>
        </div>
      ) : (
        <button
          className="btn btn--ghost workflow-status-add-btn"
          onClick={() => { setAdding(true); setError(""); }}
        >
          <Plus size={14} />
          Add status
        </button>
      )}
    </div>
  );
}
