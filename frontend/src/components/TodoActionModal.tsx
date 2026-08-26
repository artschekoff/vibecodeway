import { useEffect, useState } from "react";
import { DraftTodoMessage } from "../../wailsjs/go/main/App";
import { ProseEditor } from "./ProseEditor";

interface Props {
  kind: string;
  title: string;
  refKey: string;
  rationale: string;
  actionLabel: string;
  onClose: () => void;
}

export function TodoActionModal({ kind, title, refKey, rationale, actionLabel, onClose }: Props) {
  const [text, setText] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    setError(null);
    DraftTodoMessage(kind, title, refKey, rationale)
      .then((msg) => {
        if (alive) {
          setText(msg);
          setLoading(false);
        }
      })
      .catch((err) => {
        if (alive) {
          // Wails rejects with a plain string, not an Error — coerce whatever we got.
          setError(err instanceof Error ? err.message : String(err));
          setLoading(false);
        }
      });
    return () => {
      alive = false;
    };
  }, [kind, title, refKey, rationale]);

  const copy = () => {
    void navigator.clipboard?.writeText(text);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1500);
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="todo-action-title"
      onClick={onClose}
      style={{ position: "fixed", inset: 0, background: "rgba(0,0,0,0.4)", display: "flex", alignItems: "center", justifyContent: "center", zIndex: 50 }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{ background: "var(--surface)", border: "1px solid var(--border)", borderRadius: 8, padding: 20, maxWidth: 640, width: "90%", maxHeight: "80vh", overflowY: "auto" }}
      >
        <div id="todo-action-title" style={{ fontSize: 12, fontWeight: 600, marginBottom: 10 }}>
          {actionLabel}
        </div>
        {loading ? (
          <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 16 }}>Drafting…</div>
        ) : (
          <ProseEditor
            value={text}
            onChange={(e) => setText(e.target.value)}
            rows={8}
            style={{ width: "100%", lineHeight: 1.6, marginBottom: 16, color: "var(--text)", background: "var(--surface2)", border: "1px solid var(--border)", borderRadius: 6, padding: "8px 10px", resize: "vertical" }}
          />
        )}
        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          <button className="btn-primary" style={{ fontSize: 11, padding: "4px 12px" }} disabled={loading || !text} onClick={copy}>
            {copied ? "Copied" : "Copy"}
          </button>
          <button aria-label="Close draft" className="btn-secondary" style={{ fontSize: 11, padding: "4px 10px", marginLeft: "auto" }} onClick={onClose}>
            Close
          </button>
        </div>
        {error && <div style={{ fontSize: 11, color: "#dc2626", marginTop: 8 }}>{error}</div>}
      </div>
    </div>
  );
}
