import { useState, useEffect } from "react";
import { SaveAssignmentOutput } from "../../wailsjs/go/main/App";
import type { PersonAssignment, ReasoningChainStep } from "../types/index";

interface Props {
  pa: PersonAssignment;
  steps: ReasoningChainStep[];
  onUpdated: (pa: PersonAssignment) => void;
}

export function PersonAssignmentDetail({ pa, steps, onUpdated }: Props) {
  const [outputDraft, setOutputDraft] = useState(pa.body);
  const [isSaving, setIsSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  useEffect(() => {
    setOutputDraft(pa.body);
  }, [pa.id]);

  // Use the current step for PROMPT and SOURCE. Fall back to last step if no isCurrent.
  const currentStep = steps.find((s) => s.isCurrent) ?? steps[steps.length - 1];

  const handleSave = async () => {
    setIsSaving(true);
    setSaveError(null);
    try {
      const updated = (await SaveAssignmentOutput(pa.id, outputDraft)) as PersonAssignment;
      onUpdated(updated);
    } catch (err) {
      setSaveError(String(err));
    } finally {
      setIsSaving(false);
    }
  };

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
            fontSize: 10,
            fontWeight: 700,
            color: "var(--text-muted)",
            textTransform: "uppercase",
            letterSpacing: "0.06em",
            marginBottom: 6,
          }}
        >
          Prompt
        </div>
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
      </section>

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
    </div>
  );
}
