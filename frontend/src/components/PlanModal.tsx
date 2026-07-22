import { useEffect, useState } from "react";
import { UpsertContextText, OpenPlanInTerminal } from "../../wailsjs/go/main/App";
import type { store } from "../../wailsjs/go/models";

interface Props {
  plan: store.ContextPlan;
  contextId: string;
  onClose: () => void;
}

export function PlanModal({ plan, contextId, onClose }: Props) {
  const [copyState, setCopyState] = useState<"idle" | "copied" | "error">("idle");
  const [saveState, setSaveState] = useState<"idle" | "saving" | "saved" | "error">("idle");
  const [saveError, setSaveError] = useState<string | null>(null);
  const [openState, setOpenState] = useState<"idle" | "opened" | "error">("idle");
  const [openError, setOpenError] = useState<string | null>(null);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  const handleCopy = () => {
    navigator.clipboard
      .writeText(plan.body)
      .then(() => {
        setCopyState("copied");
        setTimeout(() => setCopyState((s) => (s === "copied" ? "idle" : s)), 1500);
      })
      .catch(() => {
        setCopyState("error");
        setTimeout(() => setCopyState((s) => (s === "error" ? "idle" : s)), 1500);
      });
  };

  const handleSave = async () => {
    setSaveState("saving");
    setSaveError(null);
    try {
      await UpsertContextText("", contextId, "Plan", plan.body);
      setSaveState("saved");
    } catch (err) {
      setSaveState("error");
      setSaveError(err instanceof Error ? err.message : "Unknown error");
    }
  };

  const handleOpen = async () => {
    setOpenState("idle");
    setOpenError(null);
    try {
      await OpenPlanInTerminal(contextId, plan.id);
      setOpenState("opened");
    } catch (err) {
      setOpenState("error");
      setOpenError(err instanceof Error ? err.message : "Unknown error");
    }
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="plan-modal-title"
      onClick={onClose}
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
          maxWidth: 640,
          width: "90%",
          maxHeight: "80vh",
          overflowY: "auto",
        }}
      >
        <div id="plan-modal-title" style={{ fontSize: 12, fontWeight: 600, marginBottom: 10 }}>
          Generated plan
        </div>
        <pre
          style={{
            whiteSpace: "pre-wrap",
            fontSize: 12,
            lineHeight: 1.6,
            marginBottom: 16,
            color: "var(--text)",
            fontFamily: "inherit",
            background: "var(--surface2)",
            borderRadius: 6,
            padding: "8px 10px",
            maxHeight: "50vh",
            overflowY: "auto",
          }}
        >
          {plan.body}
        </pre>
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
          <button
            className="btn-secondary"
            style={{ fontSize: 11, padding: "4px 10px" }}
            onClick={handleCopy}
          >
            {copyState === "copied" ? "Copied" : copyState === "error" ? "Copy failed" : "Copy"}
          </button>
          <button
            className="btn-secondary"
            style={{ fontSize: 11, padding: "4px 10px" }}
            disabled={saveState === "saving"}
            onClick={() => void handleSave()}
          >
            {saveState === "saving"
              ? "Saving…"
              : saveState === "saved"
                ? "Saved"
                : saveState === "error"
                  ? "Save failed"
                  : "Save to context"}
          </button>
          <button
            className="btn-secondary"
            style={{ fontSize: 11, padding: "4px 10px" }}
            onClick={() => void handleOpen()}
          >
            {openState === "opened"
              ? "Opened"
              : openState === "error"
                ? "Open failed"
                : "Open in terminal"}
          </button>
          <button
            aria-label="Close plan"
            className="btn-secondary"
            style={{ fontSize: 11, padding: "4px 10px", marginLeft: "auto" }}
            onClick={onClose}
          >
            Close
          </button>
        </div>
        {saveError && (
          <div style={{ fontSize: 11, color: "#dc2626", marginTop: 8 }}>{saveError}</div>
        )}
        {openError && (
          <div style={{ fontSize: 11, color: "#dc2626", marginTop: 8 }}>{openError}</div>
        )}
      </div>
    </div>
  );
}
