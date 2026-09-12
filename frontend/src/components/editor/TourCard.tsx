import { useEffect, useState } from "react";
import { X, ChevronLeft, ChevronRight, Check, Undo2, Pencil } from "lucide-react";
import { ReadRepoFile, SetCodeTourStep, SetCodeTourDecision, StageFile, DiscardFile, ConfirmAction } from "../../../wailsjs/go/main/App";
import type { CodeTour } from "../../types";

type Decision = "staged" | "discarded" | "edited" | "skipped";
type AnchorStatus = "checking" | "ok" | "lost" | "missing";

type Props = {
  tour: CodeTour;
  repoName: string;
  onClose: () => void;
  // Same signature as EditorWorkspace's openFile — reused so the step's file
  // opens in the existing DiffEditorTab flow.
  onOpenFile: (repoPath: string, repoName: string, path: string, status: string, baseMode: "branch" | "head") => void;
  // Tells the parent which (repoPath, path, anchor) to highlight once the diff
  // editor for that file is mounted. Only called when the anchor was actually
  // found in the current file content.
  onLocateAnchor: (repoPath: string, path: string, anchor: string) => void;
};

const cardStyle: React.CSSProperties = {
  position: "absolute", bottom: 12, right: 12, width: 320, maxHeight: "55%",
  display: "flex", flexDirection: "column",
  background: "var(--surface)", border: "1px solid var(--border)", borderRadius: "var(--radius-sm)",
  boxShadow: "0 4px 16px rgba(0,0,0,0.35)", zIndex: 20, fontSize: 12, color: "var(--text)", overflow: "hidden",
};
const iconBtnStyle: React.CSSProperties = {
  display: "flex", alignItems: "center", background: "none", border: "none", cursor: "pointer",
  color: "var(--text-muted)", padding: 2, flexShrink: 0,
};
const navBtnStyle: React.CSSProperties = {
  display: "flex", alignItems: "center", background: "none", border: "1px solid var(--border)", borderRadius: 4,
  cursor: "pointer", color: "var(--text)", padding: "2px 4px",
};
const actionBtnStyle: React.CSSProperties = {
  display: "flex", alignItems: "center", background: "none", border: "1px solid var(--border)", borderRadius: 4,
  cursor: "pointer", color: "var(--text)", padding: "2px 6px",
};
const badgeStyle = (color: string): React.CSSProperties => ({
  display: "inline-block", fontSize: 10, fontWeight: 600, color, background: `${color}22`,
  border: `1px solid ${color}`, borderRadius: 4, padding: "1px 6px", marginBottom: 6,
});

function decisionLabel(d: string): string {
  switch (d) {
    case "staged": return "Staged";
    case "discarded": return "Discarded";
    case "edited": return "Edited";
    case "skipped": return "Skipped";
    default: return d;
  }
}

export function TourCard({ tour, repoName, onClose, onOpenFile, onLocateAnchor }: Props) {
  const [stepIndex, setStepIndex] = useState(tour.stepIndex ?? 0);
  const [decisions, setDecisions] = useState<Record<number, string>>(tour.decisions ?? {});
  const [anchorStatus, setAnchorStatus] = useState<AnchorStatus>("checking");
  const [actionError, setActionError] = useState("");

  const steps = tour.steps ?? [];
  const step = steps[stepIndex];

  // Reset local position/decisions whenever the tour identity itself changes
  // (a brand-new tour started, or GetCodeTour returned a fresh row on resume).
  useEffect(() => {
    setStepIndex(tour.stepIndex ?? 0);
    setDecisions(tour.decisions ?? {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tour.id]);

  // On every step change: open the file, then check whether the anchor still
  // exists in the current content. Found -> tell the parent to highlight it.
  // Not found -> "anchor lost" badge, file still opens at the top. File itself
  // unreadable (missing/renamed) -> step marked broken.
  useEffect(() => {
    if (!step) return;
    let active = true;
    setAnchorStatus("checking");
    setActionError("");
    void ReadRepoFile(tour.repoPath, step.file)
      .then((content) => {
        if (!active) return;
        onOpenFile(tour.repoPath, repoName, step.file, "modified", "head");
        const idx = content.indexOf(step.anchor);
        if (idx === -1) {
          setAnchorStatus("lost");
        } else {
          setAnchorStatus("ok");
          onLocateAnchor(tour.repoPath, step.file, step.anchor);
        }
      })
      .catch(() => {
        if (active) setAnchorStatus("missing");
      });
    return () => { active = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stepIndex, tour.id, tour.repoPath, step?.file, step?.anchor]);

  if (!step) return null;

  const broken = anchorStatus === "missing";
  const decision = decisions[stepIndex];

  const goto = (idx: number) => {
    if (idx < 0 || idx >= steps.length) return;
    setActionError("");
    setStepIndex(idx);
    void SetCodeTourStep(tour.id, idx);
  };

  const recordDecision = (d: Decision) => {
    setDecisions((prev) => ({ ...prev, [stepIndex]: d }));
    void SetCodeTourDecision(tour.id, stepIndex, d);
  };

  // No optimistic success (mirrors ChangedFilesPanel's act()): the decision is
  // only recorded once the binding actually resolves. A rejection surfaces
  // inline instead of silently persisting a false "Staged"/"Discarded" badge.
  const handleStage = async () => {
    if (broken) return;
    setActionError("");
    try {
      await StageFile(tour.repoPath, step.file);
      recordDecision("staged");
    } catch (e) {
      setActionError(e instanceof Error ? e.message : String(e));
    }
  };

  const handleDiscard = async () => {
    if (broken) return;
    if (!(await ConfirmAction("Discard changes", `Discard changes to ${step.file}? This cannot be undone.`))) return;
    setActionError("");
    try {
      await DiscardFile(tour.repoPath, step.file);
      recordDecision("discarded");
    } catch (e) {
      setActionError(e instanceof Error ? e.message : String(e));
    }
  };

  const handleEdit = () => {
    if (broken) return;
    onOpenFile(tour.repoPath, repoName, step.file, "modified", "head");
    if (anchorStatus === "ok") onLocateAnchor(tour.repoPath, step.file, step.anchor);
    recordDecision("edited");
  };

  return (
    <div style={cardStyle} role="dialog" aria-label="Code tour">
      <div style={{ display: "flex", alignItems: "center", gap: 6, padding: "8px 10px", borderBottom: "1px solid var(--border)" }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontSize: 12, fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={tour.title}>
            {tour.title || "Code tour"}
          </div>
          <div style={{ fontSize: 10, color: "var(--text-muted)" }}>Step {stepIndex + 1} of {steps.length}</div>
        </div>
        <button aria-label="close tour" title="Close tour" onClick={onClose} style={iconBtnStyle}>
          <X size={13} />
        </button>
      </div>

      <div style={{ padding: "8px 10px", overflowY: "auto", flex: 1, minHeight: 0 }}>
        <div
          style={{ fontSize: 11, fontFamily: "monospace", color: "var(--text-muted)", marginBottom: 6, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
          title={step.file}
        >
          {step.file}
        </div>

        {broken && <div role="status" style={badgeStyle("#f85149")}>File missing — step broken</div>}
        {!broken && anchorStatus === "lost" && <div role="status" style={badgeStyle("#d29922")}>Anchor lost</div>}
        {decision && <div role="status" style={badgeStyle("#3fb950")}>{decisionLabel(decision)}</div>}
        {actionError && <div role="alert" style={{ fontSize: 11, color: "#f85149", marginBottom: 6 }}>{actionError}</div>}

        <div style={{ marginBottom: 8 }}>
          <div style={{ fontSize: 10, textTransform: "uppercase", letterSpacing: 0.3, color: "var(--text-muted)", marginBottom: 2 }}>What</div>
          <div>{step.what}</div>
        </div>
        <div>
          <div style={{ fontSize: 10, textTransform: "uppercase", letterSpacing: 0.3, color: "var(--text-muted)", marginBottom: 2 }}>Why</div>
          <div>{step.why}</div>
        </div>

        {/* Task 6 (Q&A): input + inline answer history slot goes here, at the
            bottom of the scrollable content area, below What/Why. */}
      </div>

      <div style={{ display: "flex", alignItems: "center", gap: 4, padding: "6px 10px", borderTop: "1px solid var(--border)" }}>
        <button aria-label="previous step" title="Previous step" disabled={stepIndex === 0} onClick={() => goto(stepIndex - 1)} style={navBtnStyle}>
          <ChevronLeft size={14} />
        </button>
        <button aria-label="next step" title="Next step" disabled={stepIndex >= steps.length - 1} onClick={() => goto(stepIndex + 1)} style={navBtnStyle}>
          <ChevronRight size={14} />
        </button>
        <div style={{ flex: 1 }} />
        <button aria-label="stage step" title="Stage this change" disabled={broken} onClick={() => void handleStage()} style={actionBtnStyle}>
          <Check size={13} />
        </button>
        <button aria-label="discard step" title="Discard this change" disabled={broken} onClick={() => void handleDiscard()} style={actionBtnStyle}>
          <Undo2 size={13} />
        </button>
        <button aria-label="edit step" title="Edit in place" disabled={broken} onClick={handleEdit} style={actionBtnStyle}>
          <Pencil size={13} />
        </button>
      </div>
    </div>
  );
}
