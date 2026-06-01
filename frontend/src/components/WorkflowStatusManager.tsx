import { useEffect, useRef, useState } from "react";
import { Pencil, Trash2, Check, X, Plus } from "lucide-react";
import {
  ListWorkflowStatuses,
  CreateWorkflowStatus,
  UpdateWorkflowStatus,
  DeleteWorkflowStatus,
} from "../../wailsjs/go/main/App";
import type { WorkflowStatus } from "../types";

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
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const confirmTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [adding, setAdding] = useState(false);
  const [newLabel, setNewLabel] = useState("");
  const [newSlug, setNewSlug] = useState("");
  const [slugManual, setSlugManual] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    ListWorkflowStatuses().then(setStatuses).catch(console.error);
  }, []);

  async function saveEdit(id: string, slug: string) {
    try {
      const updated = await UpdateWorkflowStatus(id, slug, editLabel.trim(), "");
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
      const created = await CreateWorkflowStatus(slug, label, "");
      setStatuses((prev) => [...prev, created]);
      setAdding(false);
      setNewLabel("");
      setNewSlug("");
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
                <input
                  className="workflow-status-input"
                  value={editLabel}
                  onChange={(e) => setEditLabel(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") saveEdit(st.id, st.slug);
                    if (e.key === "Escape") setEditingId(null);
                  }}
                  autoFocus
                />
                <button className="icon-btn" onClick={() => saveEdit(st.id, st.slug)} title="Save">
                  <Check size={14} />
                </button>
                <button className="icon-btn" onClick={() => setEditingId(null)} title="Cancel">
                  <X size={14} />
                </button>
              </>
            ) : (
              <>
                <span className="workflow-status-label">{st.label}</span>
                <span className="workflow-status-slug">{st.slug}</span>
                {st.isSystem && <span className="workflow-status-system-badge">system</span>}
                <div className="workflow-status-actions">
                  <button
                    className="icon-btn"
                    onClick={() => { setEditingId(st.id); setEditLabel(st.label); setError(""); }}
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
          </div>
          <div className="workflow-status-add-actions">
            <button className="btn btn--primary" onClick={handleAdd}>Add</button>
            <button
              className="btn btn--ghost"
              onClick={() => {
                setAdding(false);
                setNewLabel("");
                setNewSlug("");
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
