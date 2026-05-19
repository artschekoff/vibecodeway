import React, { useState, useCallback } from "react";
import {
  RunSingleAssignmentStep,
  CommitStepOutput,
  SetCurrentAssignmentStep,
} from "../../wailsjs/go/main/App";
import type { PersonAssignment, ReasoningChainStep } from "../types/index";

interface Props {
  personAssignment: PersonAssignment;
  steps: ReasoningChainStep[];
  onClose: () => void;
  onStepsChanged: (updated: PersonAssignment, steps: ReasoningChainStep[]) => void;
}

type PropagationState =
  | { phase: "idle" }
  | { phase: "editing"; stepIndex: number; draftOutput: string }
  | {
      phase: "propagating";
      editedAtIndex: number;
      currentIndex: number;
      prevOutput: string;
      previewOutput: string | null;
      isLoading: boolean;
      committed: Array<{ stepIndex: number; inputState: string; outputState: string }>;
    };

const SOURCE_LABELS: Record<string, string> = {
  initial: "Initial generation",
  message: "New message",
  instruction: "Manual instruction",
};

const SOURCE_COLORS: Record<string, string> = {
  initial: "#6366f1",
  message: "#0ea5e9",
  instruction: "#8b5cf6",
};

function SourceBadge({ sourceType, messageContent }: { sourceType: string; messageContent: string }) {
  // Try to infer provider from message content prefix like "[slack: ...]"
  const providerMatch = messageContent.match(/^\[([^\]]+):/);
  const provider = providerMatch ? providerMatch[1].toLowerCase() : sourceType;
  const color = SOURCE_COLORS[sourceType] ?? "#64748b";
  return (
    <span style={{ background: color, color: "#fff", borderRadius: 4, padding: "2px 7px", fontSize: 10, fontWeight: 600, letterSpacing: "0.03em" }}>
      {providerMatch ? provider : SOURCE_LABELS[sourceType] ?? sourceType}
    </span>
  );
}

export function ReasoningChainGraph({ personAssignment, steps, onClose, onStepsChanged }: Props) {
  const [state, setState] = useState<PropagationState>({ phase: "idle" });
  const [promotingId, setPromotingId] = useState<string | null>(null);

  const paId = personAssignment.id;

  // ── Editing ─────────────────────────────────────────────────────────────
  const startEdit = (stepIndex: number) => {
    setState({ phase: "editing", stepIndex, draftOutput: steps[stepIndex].outputState });
  };

  const cancelEdit = () => setState({ phase: "idle" });

  const saveAndPropagate = useCallback(async (editedStepIndex: number, newOutput: string) => {
    const nextIndex = editedStepIndex + 1;
    const hasNext = nextIndex < steps.length;

    if (!hasNext) {
      // Last step — commit immediately, no propagation needed.
      const updated = await CommitStepOutput(paId, editedStepIndex, steps[editedStepIndex].inputState, newOutput) as PersonAssignment;
      onStepsChanged(updated, steps.map((s, i) => i === editedStepIndex ? { ...s, outputState: newOutput } : s));
      setState({ phase: "idle" });
      return;
    }

    // Commit this step, then begin propagation for the next one.
    await CommitStepOutput(paId, editedStepIndex, steps[editedStepIndex].inputState, newOutput);

    setState({
      phase: "propagating",
      editedAtIndex: editedStepIndex,
      currentIndex: nextIndex,
      prevOutput: newOutput,
      previewOutput: null,
      isLoading: true,
      committed: [{ stepIndex: editedStepIndex, inputState: steps[editedStepIndex].inputState, outputState: newOutput }],
    });

    // Load preview for step nextIndex
    try {
      const preview = await RunSingleAssignmentStep(paId, nextIndex, newOutput) as string;
      setState(s => s.phase === "propagating" ? { ...s, previewOutput: preview, isLoading: false } : s);
    } catch (e) {
      setState(s => s.phase === "propagating" ? { ...s, previewOutput: `(preview failed: ${String(e)})`, isLoading: false } : s);
    }
  }, [paId, steps, onStepsChanged]);

  const confirmCurrentStep = useCallback(async () => {
    if (state.phase !== "propagating") return;
    const { currentIndex, prevOutput, previewOutput, committed } = state;
    if (!previewOutput) return;

    const newCommitted = [...committed, { stepIndex: currentIndex, inputState: prevOutput, outputState: previewOutput }];
    const nextIndex = currentIndex + 1;
    const hasNext = nextIndex < steps.length;

    // Commit this step
    const updated = await CommitStepOutput(paId, currentIndex, prevOutput, previewOutput) as PersonAssignment;

    if (!hasNext) {
      // End of chain — done
      onStepsChanged(updated, steps);
      setState({ phase: "idle" });
      return;
    }

    // Proceed to next step
    setState({
      phase: "propagating",
      editedAtIndex: state.editedAtIndex,
      currentIndex: nextIndex,
      prevOutput: previewOutput,
      previewOutput: null,
      isLoading: true,
      committed: newCommitted,
    });

    try {
      const preview = await RunSingleAssignmentStep(paId, nextIndex, previewOutput) as string;
      setState(s => s.phase === "propagating" ? { ...s, previewOutput: preview, isLoading: false } : s);
    } catch (e) {
      setState(s => s.phase === "propagating" ? { ...s, previewOutput: `(preview failed: ${String(e)})`, isLoading: false } : s);
    }
  }, [state, paId, steps, onStepsChanged]);

  const stopPropagation = useCallback(async () => {
    if (state.phase !== "propagating") return;
    // Steps already committed up to currentIndex-1 (each confirmCurrentStep saved them).
    // Just reload and close propagation.
    const updated = await CommitStepOutput(paId, state.currentIndex - 1, state.committed[state.committed.length - 1]?.inputState ?? "", state.prevOutput) as PersonAssignment;
    onStepsChanged(updated, steps);
    setState({ phase: "idle" });
  }, [state, paId, steps, onStepsChanged]);

  // ── Promote step to current ──────────────────────────────────────────────
  const promoteStep = useCallback(async (stepId: string) => {
    setPromotingId(stepId);
    try {
      const updated = await SetCurrentAssignmentStep(paId, stepId) as PersonAssignment;
      onStepsChanged(updated, steps.map(s => ({ ...s, isCurrent: s.id === stepId })));
    } finally {
      setPromotingId(null);
    }
  }, [paId, steps, onStepsChanged]);

  // ── Propagation wizard overlay ───────────────────────────────────────────
  const renderPropagationWizard = () => {
    if (state.phase !== "propagating") return null;
    const { currentIndex, prevOutput, previewOutput, isLoading } = state;
    const step = steps[currentIndex];
    if (!step) return null;

    return (
      <div style={{
        position: "fixed", inset: 0, zIndex: 1100,
        background: "rgba(0,0,0,0.55)",
        display: "flex", alignItems: "center", justifyContent: "center",
      }}>
        <div style={{ background: "var(--surface)", border: "1px solid var(--border)", borderRadius: 10, padding: 24, maxWidth: 600, width: "90%", display: "flex", flexDirection: "column", gap: 14 }}>
          <div style={{ fontSize: 13, fontWeight: 600, color: "var(--text)" }}>
            Rebuild step {currentIndex + 1} of {steps.length}?
          </div>
          <div style={{ fontSize: 11, color: "var(--text-muted)" }}>
            Based on the edited output, here's what the AI would produce for:
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <SourceBadge sourceType={step.sourceType} messageContent={step.messageContent} />
            <span style={{ fontSize: 11, color: "var(--text-muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {(step.messageContent || step.userInstruction || "").slice(0, 100)}
            </span>
          </div>
          <div style={{ background: "var(--surface2)", border: "1px solid var(--border)", borderRadius: 6, padding: 10, maxHeight: 200, overflowY: "auto" }}>
            {isLoading
              ? <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--text-muted)", fontSize: 12 }}>
                  <span style={{ width: 12, height: 12, borderRadius: "50%", border: "2px solid rgba(255,255,255,0.2)", borderTopColor: "var(--accent)", display: "inline-block", animation: "spin 0.6s linear infinite" }} />
                  Generating preview…
                </div>
              : <pre style={{ whiteSpace: "pre-wrap", fontSize: 11, margin: 0, color: "var(--text)", lineHeight: 1.5 }}>{previewOutput}</pre>
            }
          </div>
          <div style={{ display: "flex", gap: 8 }}>
            <button className="btn-primary" style={{ fontSize: 12, padding: "4px 14px" }} disabled={isLoading || !previewOutput} onClick={() => void confirmCurrentStep()}>
              Confirm →
            </button>
            <button className="btn-secondary" style={{ fontSize: 12, padding: "4px 12px" }} onClick={() => void stopPropagation()}>
              Stop here
            </button>
          </div>
        </div>
      </div>
    );
  };

  // ── Main graph render ────────────────────────────────────────────────────
  return (
    <>
      {/* Full-screen modal */}
      <div style={{
        position: "fixed", inset: 0, zIndex: 1000,
        background: "var(--background, #0f1117)",
        display: "flex", flexDirection: "column",
      }}>
        {/* Header */}
        <div style={{ display: "flex", alignItems: "center", gap: 12, padding: "14px 20px", borderBottom: "1px solid var(--border)", flexShrink: 0 }}>
          <div style={{ fontWeight: 600, fontSize: 14, color: "var(--text)" }}>
            Reasoning Chain — {personAssignment.name}
          </div>
          <span style={{ fontSize: 11, color: "var(--text-muted)" }}>{steps.length} steps</span>
          <button
            style={{ marginLeft: "auto", background: "none", border: "1px solid var(--border)", borderRadius: 6, padding: "4px 12px", cursor: "pointer", fontSize: 12, color: "var(--text)" }}
            onClick={onClose}
          >
            Close
          </button>
        </div>

        {/* Scrollable chain */}
        <div style={{ flex: 1, overflowY: "auto", padding: "20px 24px", display: "flex", flexDirection: "column", gap: 0 }}>
          {steps.map((step, idx) => {
            const isEditing = state.phase === "editing" && state.stepIndex === idx;
            const draftOutput = isEditing && state.phase === "editing" ? state.draftOutput : step.outputState;

            return (
              <div key={step.id} style={{ display: "flex", flexDirection: "column", gap: 0 }}>
                {/* Step header */}
                <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
                  <span style={{ fontSize: 10, color: "var(--text-muted)", fontWeight: 500 }}>Step {idx}</span>
                  <SourceBadge sourceType={step.sourceType} messageContent={step.messageContent} />
                  {step.isCurrent && (
                    <span style={{ fontSize: 10, color: "var(--accent)", fontWeight: 700, background: "rgba(99,102,241,0.12)", padding: "1px 6px", borderRadius: 3 }}>CURRENT</span>
                  )}
                  {step.qualityScore != null && (
                    <span title={step.qualityNotes ?? ""} style={{
                      fontSize: 9, fontWeight: 600, padding: "1px 6px", borderRadius: 3,
                      background: step.qualityScore >= 0.7 ? "rgba(34,197,94,0.15)" : step.qualityScore >= 0.4 ? "rgba(234,179,8,0.15)" : "rgba(239,68,68,0.15)",
                      color: step.qualityScore >= 0.7 ? "#16a34a" : step.qualityScore >= 0.4 ? "#ca8a04" : "#dc2626",
                      cursor: "help",
                    }}>
                      ★ {(step.qualityScore * 10).toFixed(0)}/10
                    </span>
                  )}
                  {step.promptVersion && (
                    <span style={{ fontSize: 9, color: "var(--text-muted)", fontFamily: "monospace" }}>
                      {step.promptVersion}
                    </span>
                  )}
                  {!step.isCurrent && (
                    <button
                      style={{ fontSize: 10, color: "var(--accent)", background: "none", border: "none", cursor: "pointer", padding: 0, marginLeft: "auto" }}
                      disabled={promotingId === step.id}
                      onClick={() => void promoteStep(step.id)}
                    >
                      {promotingId === step.id ? "Setting…" : "Set as current"}
                    </button>
                  )}
                </div>

                {/* Message card */}
                <div style={{ border: "1px solid var(--border)", borderRadius: 6, background: "var(--surface2)", padding: "10px 12px", fontSize: 11, color: "var(--text-muted)", position: "relative" }}>
                  <div style={{ fontSize: 10, fontWeight: 600, color: "var(--text-muted)", marginBottom: 4, textTransform: "uppercase", letterSpacing: "0.05em" }}>
                    {step.sourceType === "instruction" ? "User instruction" : "Message"}
                  </div>
                  <pre style={{ whiteSpace: "pre-wrap", margin: 0, fontFamily: "inherit", fontSize: 11, maxHeight: 120, overflowY: "auto", color: "var(--text)" }}>
                    {(step.messageContent || step.userInstruction || "").slice(0, 500)}
                    {(step.messageContent || step.userInstruction || "").length > 500 ? "…" : ""}
                  </pre>
                </div>

                {/* Arrow */}
                <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: 28, color: "var(--text-muted)", fontSize: 18, flexShrink: 0 }}>↓</div>

                {/* Output card */}
                <div style={{ border: `1px solid ${step.isCurrent ? "var(--accent)" : "var(--border)"}`, borderRadius: 6, background: "var(--surface)", padding: "10px 12px", position: "relative" }}>
                  <div style={{ display: "flex", alignItems: "center", marginBottom: 6, gap: 8 }}>
                    <span style={{ fontSize: 10, fontWeight: 600, color: "var(--text-muted)", textTransform: "uppercase", letterSpacing: "0.05em" }}>Output</span>
                    <span style={{ fontSize: 10, color: "var(--text-muted)", marginLeft: "auto" }}>
                      {new Date(step.createdAt).toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })}
                    </span>
                    {!isEditing && state.phase === "idle" && (
                      <button
                        style={{ fontSize: 10, color: "var(--accent)", background: "none", border: "1px solid var(--accent)", borderRadius: 4, cursor: "pointer", padding: "2px 8px" }}
                        onClick={() => startEdit(idx)}
                      >
                        Edit
                      </button>
                    )}
                  </div>

                  {isEditing ? (
                    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                      <textarea
                        style={{ width: "100%", minHeight: 180, resize: "vertical", fontFamily: "inherit", fontSize: 11, textAlign: "left", padding: "8px", boxSizing: "border-box", background: "var(--surface2)", border: "1px solid var(--border)", borderRadius: 4, color: "var(--text)", lineHeight: 1.5 }}
                        value={state.draftOutput}
                        onChange={e => setState(s => s.phase === "editing" ? { ...s, draftOutput: e.target.value } : s)}
                      />
                      <div style={{ display: "flex", gap: 8 }}>
                        <button
                          className="btn-primary"
                          style={{ fontSize: 11, padding: "4px 12px" }}
                          onClick={() => void saveAndPropagate(idx, state.draftOutput)}
                        >
                          {idx === steps.length - 1 ? "Save" : "Save & Propagate →"}
                        </button>
                        <button
                          className="btn-secondary"
                          style={{ fontSize: 11, padding: "4px 10px" }}
                          onClick={cancelEdit}
                        >
                          Cancel
                        </button>
                      </div>
                    </div>
                  ) : (
                    <pre style={{ whiteSpace: "pre-wrap", margin: 0, fontFamily: "inherit", fontSize: 11, maxHeight: 160, overflowY: "auto", color: "var(--text)", lineHeight: 1.5 }}>
                      {draftOutput.slice(0, 600)}
                      {draftOutput.length > 600 ? "…" : ""}
                    </pre>
                  )}
                </div>

                {/* Connector to next step */}
                {idx < steps.length - 1 && (
                  <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: 32, color: "var(--text-muted)", fontSize: 18 }}>↓</div>
                )}
              </div>
            );
          })}

          {steps.length === 0 && (
            <div style={{ color: "var(--text-muted)", fontSize: 13, textAlign: "center", marginTop: 40 }}>
              No reasoning steps yet. Generate an assignment first.
            </div>
          )}
        </div>
      </div>

      {/* Propagation wizard overlay */}
      {renderPropagationWizard()}
    </>
  );
}
