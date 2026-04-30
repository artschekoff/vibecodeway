import { useState } from "react";
import { Check, Edit2, RotateCcw } from "lucide-react";
import { SetUserWorkflowOverrides } from "../../wailsjs/go/main/App";
import type { WorkflowSummary, WorkflowStatusValue } from "../types";
import WorkflowStatusBadge from "./WorkflowStatusBadge";

const MANUAL_STATUSES: { value: WorkflowStatusValue; label: string }[] = [
  { value: "not_started",     label: "Not started" },
  { value: "scoping",         label: "Scoping" },
  { value: "in_progress",     label: "In progress" },
  { value: "needs_attention",  label: "Needs attention" },
  { value: "ready",           label: "Ready" },
];

interface Props {
  contextId: string;
  summary: WorkflowSummary;
  onUpdated: (summary: WorkflowSummary) => void;
}

export default function WorkflowProposalCard({ contextId, summary, onUpdated }: Props) {
  const [editingField, setEditingField] = useState<"status" | "comment" | "nextStep" | null>(null);
  const [draftStatus,   setDraftStatus]   = useState(summary.userStatusOverride  ?? summary.aiProposedStatus);
  const [draftComment,  setDraftComment]  = useState(summary.userCommentOverride  ?? summary.aiProposedComment  ?? "");
  const [draftNextStep, setDraftNextStep] = useState(summary.userNextStepOverride ?? summary.aiProposedNextStep ?? "");
  const [saving, setSaving] = useState(false);

  const statusLocked = summary.aiProposedStatus === "needs_attention";

  const save = async (status: string, comment: string, nextStep: string) => {
    setSaving(true);
    try {
      await SetUserWorkflowOverrides(contextId, status, comment, nextStep);
      onUpdated({
        ...summary,
        userStatusOverride:   status   || undefined,
        userCommentOverride:  comment  || undefined,
        userNextStepOverride: nextStep || undefined,
        effectiveStatus:  statusLocked ? "needs_attention" : (status || summary.aiProposedStatus),
        effectiveComment: comment  || summary.aiProposedComment  || "",
        effectiveNextStep: nextStep || summary.aiProposedNextStep || "",
      });
    } finally {
      setSaving(false);
      setEditingField(null);
    }
  };

  const clearOverride = async (field: "status" | "comment" | "nextStep") => {
    const s = field === "status"   ? "" : (summary.userStatusOverride   ?? "");
    const c = field === "comment"  ? "" : (summary.userCommentOverride  ?? "");
    const n = field === "nextStep" ? "" : (summary.userNextStepOverride ?? "");
    await save(s, c, n);
  };

  return (
    <div className="workflow-proposal-card">
      <div className="workflow-proposal-title">Workflow Status</div>

      {/* STATUS ROW */}
      <div className="workflow-proposal-row">
        <span className="workflow-proposal-label">Status</span>
        <div className="workflow-proposal-values">
          <div className="workflow-proposal-ai-value">
            <WorkflowStatusBadge status={summary.aiProposedStatus} isAI />
            <span className="workflow-proposal-source">AI proposed</span>
          </div>
          {summary.userStatusOverride && !statusLocked ? (
            <div className="workflow-proposal-override">
              <WorkflowStatusBadge status={summary.userStatusOverride} />
              <span className="workflow-proposal-source">Your override</span>
              <button className="workflow-action-btn" onClick={() => clearOverride("status")} title="Revert to AI">
                <RotateCcw size={11} />
              </button>
            </div>
          ) : !statusLocked ? (
            <button
              className="workflow-action-btn workflow-accept-btn"
              onClick={() => setEditingField("status")}
              title="Override status"
            >
              <Edit2 size={11} /> Override
            </button>
          ) : (
            <span className="workflow-status-locked-note">Status locked while attention conditions exist</span>
          )}
        </div>
        {editingField === "status" && !statusLocked && (
          <div className="workflow-edit-inline">
            <select value={draftStatus} onChange={(e) => setDraftStatus(e.target.value)} className="workflow-select">
              {MANUAL_STATUSES.map((s) => <option key={s.value} value={s.value}>{s.label}</option>)}
            </select>
            <button onClick={() => save(draftStatus, summary.userCommentOverride ?? "", summary.userNextStepOverride ?? "")} disabled={saving} className="btn-primary btn-sm">
              {saving ? "…" : <Check size={12} />}
            </button>
            <button onClick={() => setEditingField(null)} className="btn-secondary btn-sm">Cancel</button>
          </div>
        )}
      </div>

      {/* COMMENT ROW */}
      <WorkflowTextField
        label="Comment"
        aiValue={summary.aiProposedComment}
        userValue={summary.userCommentOverride}
        isEditing={editingField === "comment"}
        draft={draftComment}
        setDraft={setDraftComment}
        saving={saving}
        onEdit={() => setEditingField("comment")}
        onSave={() => save(summary.userStatusOverride ?? "", draftComment, summary.userNextStepOverride ?? "")}
        onClear={() => clearOverride("comment")}
        onCancel={() => setEditingField(null)}
      />

      {/* NEXT STEP ROW */}
      <WorkflowTextField
        label="Next step"
        aiValue={summary.aiProposedNextStep}
        userValue={summary.userNextStepOverride}
        isEditing={editingField === "nextStep"}
        draft={draftNextStep}
        setDraft={setDraftNextStep}
        saving={saving}
        onEdit={() => setEditingField("nextStep")}
        onSave={() => save(summary.userStatusOverride ?? "", summary.userCommentOverride ?? "", draftNextStep)}
        onClear={() => clearOverride("nextStep")}
        onCancel={() => setEditingField(null)}
      />
    </div>
  );
}

interface TextFieldProps {
  label: string;
  aiValue?: string;
  userValue?: string;
  isEditing: boolean;
  draft: string;
  setDraft: (v: string) => void;
  saving: boolean;
  onEdit: () => void;
  onSave: () => void;
  onClear: () => void;
  onCancel: () => void;
}

function WorkflowTextField({
  label, aiValue, userValue, isEditing, draft, setDraft,
  saving, onEdit, onSave, onClear, onCancel,
}: TextFieldProps) {
  return (
    <div className="workflow-proposal-row">
      <span className="workflow-proposal-label">{label}</span>
      <div className="workflow-proposal-values">
        {aiValue && (
          <div className="workflow-proposal-ai-value">
            <span className="workflow-text-value">{aiValue}</span>
            <span className="workflow-proposal-source">AI proposed</span>
          </div>
        )}
        {userValue ? (
          <div className="workflow-proposal-override">
            <span className="workflow-text-value workflow-text-value--override">{userValue}</span>
            <span className="workflow-proposal-source">Your override</span>
            <button className="workflow-action-btn" onClick={onClear} title="Revert to AI">
              <RotateCcw size={11} />
            </button>
            <button className="workflow-action-btn" onClick={onEdit} title="Edit">
              <Edit2 size={11} />
            </button>
          </div>
        ) : (
          <button className="workflow-action-btn workflow-accept-btn" onClick={onEdit} title="Override">
            <Edit2 size={11} /> Override
          </button>
        )}
      </div>
      {isEditing && (
        <div className="workflow-edit-inline">
          <input
            autoFocus
            type="text"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && onSave()}
            className="workflow-text-input"
            placeholder={aiValue ?? `Enter ${label.toLowerCase()}…`}
          />
          <button onClick={onSave} disabled={saving} className="btn-primary btn-sm">
            {saving ? "…" : <Check size={12} />}
          </button>
          <button onClick={onCancel} className="btn-secondary btn-sm">Cancel</button>
        </div>
      )}
    </div>
  );
}
