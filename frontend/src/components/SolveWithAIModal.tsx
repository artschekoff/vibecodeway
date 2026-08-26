import { useEffect, useState } from "react";
import { SolveWithAI } from "../../wailsjs/go/main/App";
import { ProseEditor } from "./ProseEditor";

interface Props {
  contextId: string;
  onClose: () => void;
  onOpenEditor?: () => void;
}

export function SolveWithAIModal({ contextId, onClose, onOpenEditor }: Props) {
  const [task, setTask] = useState("implement");
  const [loading, setLoading] = useState(false);
  const [done, setDone] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  const handleConfirm = async () => {
    setLoading(true);
    setError(null);
    try {
      const cmd = await SolveWithAI(contextId, task);
      await navigator.clipboard.writeText(cmd).catch(() => {});
      setDone(true);
      onOpenEditor?.();
    } catch (err) {
      // Wails rejects with a plain string, not an Error — coerce whatever we got.
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg || "Unknown error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="solve-with-ai-modal-title"
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
        <div id="solve-with-ai-modal-title" style={{ fontSize: 12, fontWeight: 600, marginBottom: 10 }}>
          Solve with AI
        </div>
        <ProseEditor
          value={task}
          onChange={(e) => setTask(e.target.value)}
          rows={4}
          style={{
            width: "100%",
            lineHeight: 1.6,
            marginBottom: 16,
            color: "var(--text)",
            background: "var(--surface2)",
            border: "1px solid var(--border)",
            borderRadius: 6,
            padding: "8px 10px",
            resize: "vertical"
          }}
        />
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
          <button
            className="btn-primary"
            style={{ fontSize: 11, padding: "4px 12px" }}
            disabled={loading}
            onClick={() => void handleConfirm()}
          >
            {loading ? "Solving…" : "Solve"}
          </button>
          <button
            aria-label="Close solve with AI"
            className="btn-secondary"
            style={{ fontSize: 11, padding: "4px 10px", marginLeft: "auto" }}
            onClick={onClose}
          >
            Close
          </button>
        </div>
        {error && (
          <div style={{ fontSize: 11, color: "#dc2626", marginTop: 8 }}>{error}</div>
        )}
        {done && (
          <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 8 }}>
            Task copied to clipboard — paste it into the terminal of the editor that just opened.
          </div>
        )}
      </div>
    </div>
  );
}
