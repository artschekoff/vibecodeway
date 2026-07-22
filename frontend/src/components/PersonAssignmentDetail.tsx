import { useState, useEffect, useRef } from "react";
import {
  SaveAssignmentOutput,
  RunAssignmentStepWithCustomPrompt,
  CommitAssignmentVersionFromCustomPrompt,
  ListAssignmentVersions,
  SetActiveAssignmentVersion,
} from "../../wailsjs/go/main/App";
import type { PersonAssignment, PersonAssignmentVersion, ReasoningChainStep } from "../types/index";
import { VersionDiffModal } from "./VersionDiffModal";
import { parseBody, toggleAt, addStep } from "../lib/taskList";

interface Props {
  pa: PersonAssignment;
  steps: ReasoningChainStep[];
  onUpdated: (pa: PersonAssignment) => void;
}

type PromptEditState =
  | { mode: "view" }
  | { mode: "editing"; draftPrompt: string }
  | { mode: "preview"; draftPrompt: string; previewOutput: string; isSending: boolean; confirmError?: string };

export function PersonAssignmentDetail({ pa, steps, onUpdated }: Props) {
  const [outputDraft, setOutputDraft] = useState(pa.body);
  const [isSaving, setIsSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [promptState, setPromptState] = useState<PromptEditState>({ mode: "view" });
  const [versions, setVersions] = useState<PersonAssignmentVersion[]>([]);
  const [versionsError, setVersionsError] = useState<string | null>(null);
  const [selectedVersionId, setSelectedVersionId] = useState<string | null>(null);
  const [isConfirming, setIsConfirming] = useState(false);
  const [versionConfirmError, setVersionConfirmError] = useState<string | undefined>(undefined);
  // Guard against double-sends while a handleSend call is still in flight.
  const isSendInFlight = useRef(false);

  useEffect(() => {
    let ignored = false;
    setOutputDraft(pa.body);
    setSelectedVersionId(null);
    setVersions([]);
    setVersionsError(null);
    void ListAssignmentVersions(pa.id).then((vs) => {
      if (!ignored) setVersions(vs as PersonAssignmentVersion[]);
    }).catch((err: unknown) => {
      if (!ignored) setVersionsError(String(err));
    });
    return () => { ignored = true; };
  }, [pa.id]);

  // Escape key handler for preview dialog
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && promptState.mode === "preview" && !promptState.isSending) {
        setPromptState({ mode: "view" });
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [promptState]);

  // Use the current step for PROMPT and SOURCE. Fall back to last step if no isCurrent.
  const currentStep = steps.find((s) => s.isCurrent) ?? steps[steps.length - 1];

  const persist = async (body: string) => {
    setIsSaving(true);
    setSaveError(null);
    try {
      const updated = (await SaveAssignmentOutput(pa.id, body)) as PersonAssignment;
      onUpdated(updated);
    } catch (err) {
      setSaveError(String(err));
    } finally {
      setIsSaving(false);
    }
  };

  const handleSave = () => persist(outputDraft);

  const handleSend = async () => {
    if (promptState.mode !== "editing") return;
    if (isSendInFlight.current) return;
    isSendInFlight.current = true;
    const draft = promptState.draftPrompt;
    const currentId = pa.id;
    setPromptState({ mode: "preview", draftPrompt: draft, previewOutput: "", isSending: true });
    try {
      const output = (await RunAssignmentStepWithCustomPrompt(
        pa.id,
        currentStep.stepIndex,
        draft
      )) as string;
      if (pa.id !== currentId) return;
      setPromptState({ mode: "preview", draftPrompt: draft, previewOutput: output, isSending: false });
    } catch (err) {
      if (pa.id !== currentId) return;
      setPromptState({
        mode: "preview",
        draftPrompt: draft,
        previewOutput: `(error: ${String(err)})`,
        isSending: false,
      });
    } finally {
      isSendInFlight.current = false;
    }
  };

  const handleConfirm = async () => {
    if (promptState.mode !== "preview" || !promptState.previewOutput) return;
    try {
      const updated = (await CommitAssignmentVersionFromCustomPrompt(
        pa.id,
        currentStep.stepIndex,
        promptState.draftPrompt,
        promptState.previewOutput
      )) as PersonAssignment;
      onUpdated(updated);
      setPromptState({ mode: "view" });
    } catch (err) {
      setPromptState((s) =>
        s.mode === "preview" ? { ...s, confirmError: String(err) } : s
      );
    }
  };

  const handleVersionConfirm = async () => {
    if (!selectedVersionId) return;
    setIsConfirming(true);
    setVersionConfirmError(undefined);
    try {
      const result = (await SetActiveAssignmentVersion(pa.id, selectedVersionId)) as PersonAssignment;
      onUpdated(result);
      setSelectedVersionId(null);
    } catch (err) {
      setVersionConfirmError(String(err));
    } finally {
      setIsConfirming(false);
    }
  };

  const selectedVersion = selectedVersionId
    ? versions.find((v) => v.id === selectedVersionId) ?? null
    : null;

  const parsedBody = parseBody(pa.body);
  const checklistLines = parsedBody.some((l) => l.kind === "task") ? parsedBody : null;

  if (!steps.length) {
    return (
      <div style={{ padding: 16, color: "var(--text-muted)", fontStyle: "italic", fontSize: 12 }}>
        No reasoning steps available.
      </div>
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12, padding: 16 }}>
      {/* SOURCE */}
      <section>
        <div
          style={{
            fontSize: 10,
            fontWeight: 700,
            color: "var(--text-muted)",
            textTransform: "uppercase",
            letterSpacing: "0.06em",
            marginBottom: 6,
          }}
        >
          Source
        </div>
        <div
          style={{
            fontSize: 12,
            color: "var(--text-muted)",
            background: "var(--surface2)",
            borderRadius: 6,
            padding: "8px 10px",
            whiteSpace: "pre-wrap",
            lineHeight: 1.5,
            maxHeight: 120,
            overflowY: "auto",
          }}
        >
          {currentStep?.messageContent ?? ""}
        </div>
      </section>

      {/* PROMPT */}
      <section>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            marginBottom: 6,
          }}
        >
          <span
            style={{
              fontSize: 10,
              fontWeight: 700,
              color: "var(--text-muted)",
              textTransform: "uppercase",
              letterSpacing: "0.06em",
            }}
          >
            Prompt
          </span>
          {promptState.mode === "view" && currentStep?.llmPrompt && (
            <button
              className="btn-secondary"
              style={{ fontSize: 10, padding: "2px 8px" }}
              onClick={() =>
                setPromptState({ mode: "editing", draftPrompt: currentStep.llmPrompt! })
              }
            >
              Edit
            </button>
          )}
        </div>

        {promptState.mode === "view" ? (
          <div
            style={{
              fontSize: 11,
              color: "var(--text-muted)",
              background: "var(--surface2)",
              borderRadius: 6,
              padding: "8px 10px",
              lineHeight: 1.5,
            }}
          >
            {currentStep?.llmPrompt ? (
              <pre
                style={{
                  margin: 0,
                  whiteSpace: "pre-wrap",
                  fontFamily: "inherit",
                  fontSize: 11,
                  maxHeight: 100,
                  overflowY: "auto",
                }}
              >
                {currentStep.llmPrompt.slice(0, 400)}
                {currentStep.llmPrompt.length > 400 ? "…" : ""}
              </pre>
            ) : (
              <span style={{ fontStyle: "italic" }}>No prompt stored</span>
            )}
          </div>
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            <textarea
              aria-label="Prompt"
              style={{
                width: "100%",
                minHeight: 160,
                resize: "vertical",
                fontFamily: "monospace",
                fontSize: 11,
                padding: "8px 10px",
                boxSizing: "border-box",
                background: "var(--surface)",
                border: "1px solid var(--border)",
                borderRadius: 6,
                color: "var(--text)",
                lineHeight: 1.4,
              }}
              value={promptState.draftPrompt}
              onChange={(e) =>
                setPromptState((s) =>
                  s.mode === "editing" ? { ...s, draftPrompt: e.target.value } : s
                )
              }
            />
            <div style={{ display: "flex", gap: 8 }}>
              <button
                className="btn-primary"
                style={{ fontSize: 11, padding: "4px 12px" }}
                disabled={promptState.mode === "preview"}
                onClick={() => void handleSend()}
              >
                Send
              </button>
              <button
                aria-label="Cancel prompt edit"
                className="btn-secondary"
                style={{ fontSize: 11, padding: "4px 10px" }}
                onClick={() => setPromptState({ mode: "view" })}
              >
                Cancel
              </button>
            </div>
          </div>
        )}
      </section>

      {/* Preview modal */}
      {promptState.mode === "preview" && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="preview-dialog-title"
          onClick={() => {
            if (!promptState.isSending) setPromptState({ mode: "view" });
          }}
          style={{
            position: "fixed",
            inset: 0,
            background: "rgba(0,0,0,0.4)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            zIndex: 50,
          }}
        >
          <div
            onClick={(e) => e.stopPropagation()}
            style={{
              background: "var(--surface)",
              border: "1px solid var(--border)",
              borderRadius: 8,
              padding: 20,
              maxWidth: 600,
              width: "90%",
              maxHeight: "80vh",
              overflowY: "auto",
            }}
          >
            <div id="preview-dialog-title" style={{ fontSize: 12, fontWeight: 600, marginBottom: 10 }}>
              New output preview
            </div>
            <pre
              style={{
                whiteSpace: "pre-wrap",
                fontSize: 12,
                lineHeight: 1.6,
                marginBottom: 16,
                color: "var(--text)",
              }}
            >
              {promptState.isSending ? "Generating…" : promptState.previewOutput}
            </pre>
            {promptState.confirmError && (
              <div style={{ fontSize: 11, color: "#dc2626", marginBottom: 8 }}>{promptState.confirmError}</div>
            )}
            <div style={{ display: "flex", gap: 8 }}>
              <button
                className="btn-primary"
                style={{ fontSize: 11, padding: "4px 12px" }}
                disabled={promptState.isSending || !promptState.previewOutput}
                onClick={() => void handleConfirm()}
              >
                Confirm
              </button>
              <button
                aria-label="Cancel preview"
                className="btn-secondary"
                style={{ fontSize: 11, padding: "4px 10px" }}
                onClick={() => setPromptState({ mode: "view" })}
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      {/* OUTPUT */}
      <section>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            marginBottom: 6,
          }}
        >
          <label
            htmlFor="pa-output"
            style={{
              fontSize: 10,
              fontWeight: 700,
              color: "var(--text-muted)",
              textTransform: "uppercase",
              letterSpacing: "0.06em",
            }}
          >
            Output
          </label>
          <button
            className="btn-primary"
            style={{ fontSize: 11, padding: "3px 12px" }}
            disabled={isSaving || outputDraft === pa.body}
            onClick={() => void handleSave()}
          >
            {isSaving ? "Saving…" : "Save"}
          </button>
        </div>
        {saveError && (
          <div style={{ fontSize: 11, color: "#dc2626", marginTop: 4 }}>{saveError}</div>
        )}
        {checklistLines && (
          <div
            style={{
              background: "var(--surface2)",
              borderRadius: 6,
              padding: "8px 10px",
              marginBottom: 8,
            }}
          >
            {(() => {
              let taskIndex = -1;
              return checklistLines.map((line, i) => {
                if (line.kind === "task") {
                  taskIndex += 1;
                  const idx = taskIndex;
                  return (
                    <label
                      key={i}
                      style={{
                        display: "flex",
                        alignItems: "flex-start",
                        gap: 6,
                        fontSize: 12,
                        lineHeight: 1.6,
                        color: "var(--text)",
                        cursor: "pointer",
                      }}
                    >
                      <input
                        type="checkbox"
                        checked={line.checked}
                        disabled={isSaving}
                        onChange={() => void persist(toggleAt(pa.body, idx))}
                      />
                      <span
                        style={{
                          textDecoration: line.checked ? "line-through" : "none",
                          color: line.checked ? "var(--text-muted)" : "var(--text)",
                        }}
                      >
                        {line.text}
                      </span>
                    </label>
                  );
                }
                return (
                  <div key={i} style={{ fontSize: 12, lineHeight: 1.6, color: "var(--text)" }}>
                    {line.raw}
                  </div>
                );
              });
            })()}
            <button
              className="btn-secondary"
              style={{ fontSize: 11, padding: "3px 10px", marginTop: 8 }}
              disabled={isSaving}
              onClick={() => void persist(addStep(pa.body))}
            >
              Add step
            </button>
          </div>
        )}
        <textarea
          id="pa-output"
          aria-label="Output"
          style={{
            width: "100%",
            minHeight: 200,
            resize: "vertical",
            fontFamily: "inherit",
            fontSize: 12,
            padding: "8px 10px",
            boxSizing: "border-box",
            background: "var(--surface)",
            border: "1px solid var(--border)",
            borderRadius: 6,
            color: "var(--text)",
            lineHeight: 1.6,
          }}
          value={outputDraft}
          onChange={(e) => setOutputDraft(e.target.value)}
        />
      </section>

      {/* VERSION SELECTOR */}
      {versionsError && (
        <div style={{ fontSize: 11, color: "#dc2626" }} role="alert">
          Failed to load versions: {versionsError}
        </div>
      )}
      {versions.length > 0 && (
        <section>
          <div
            style={{
              fontSize: 10,
              fontWeight: 700,
              color: "var(--text-muted)",
              textTransform: "uppercase",
              letterSpacing: "0.06em",
              marginBottom: 6,
            }}
          >
            Versions
          </div>
          <select
            aria-label="Versions"
            style={{
              fontSize: 12,
              padding: "4px 8px",
              background: "var(--surface)",
              border: "1px solid var(--border)",
              borderRadius: 6,
              color: "var(--text)",
              width: "100%",
            }}
            value={selectedVersionId ?? ""}
            onChange={(e) => setSelectedVersionId(e.target.value || null)}
          >
            <option value="">Select version…</option>
            {versions.map((v) => (
              <option key={v.id} value={v.id}>
                {v.createdAt}
              </option>
            ))}
          </select>
        </section>
      )}

      {/* VERSION DIFF MODAL */}
      {selectedVersion && (
        <VersionDiffModal
          currentBody={pa.body}
          selectedVersion={selectedVersion}
          onConfirm={handleVersionConfirm}
          onCancel={() => setSelectedVersionId(null)}
          isConfirming={isConfirming}
          confirmError={versionConfirmError}
        />
      )}
    </div>
  );
}
