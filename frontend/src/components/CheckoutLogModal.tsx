import { useEffect, useRef } from "react";
import Spinner from "./Spinner";

type LogLine = { line: string; stream: "stdout" | "stderr" };

interface Props {
  open: boolean;
  running: boolean;
  logs: LogLine[];
  command: string;
  exitCode: number | null;
  onClose: () => void;
}

export default function CheckoutLogModal({ open, running, logs, command, exitCode, onClose }: Props) {
  const tailRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) return;
    tailRef.current?.scrollTo({ top: tailRef.current.scrollHeight });
  }, [logs, open]);

  if (!open) return null;

  const statusLabel = running
    ? "Running…"
    : exitCode === 0
      ? "Done"
      : `Exited with code ${exitCode ?? "?"}`;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div
        className="modal"
        style={{ width: "min(720px, 90vw)", maxHeight: "80vh", display: "flex", flexDirection: "column" }}
        onClick={e => e.stopPropagation()}
      >
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 8 }}>
          <div style={{ fontWeight: 600 }}>On-checkout command</div>
          <div style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12, opacity: 0.8 }}>
            {running && <Spinner size={14} />}
            <span>{statusLabel}</span>
          </div>
        </div>

        {command && (
          <div style={{ fontSize: 12, opacity: 0.7, marginBottom: 8, fontFamily: "monospace", wordBreak: "break-all" }}>
            $ {command}
          </div>
        )}

        <div
          ref={tailRef}
          style={{
            flex: 1,
            overflow: "auto",
            background: "rgba(0,0,0,0.35)",
            border: "1px solid rgba(255,255,255,0.08)",
            borderRadius: 6,
            padding: 8,
            fontFamily: "monospace",
            fontSize: 12,
            whiteSpace: "pre-wrap",
            minHeight: 200,
          }}
        >
          {logs.length === 0 && running && <div style={{ opacity: 0.6 }}>Waiting for output…</div>}
          {logs.length === 0 && !running && <div style={{ opacity: 0.6 }}>No output.</div>}
          {logs.map((l, i) => (
            <div key={i} style={{ color: l.stream === "stderr" ? "#f87171" : undefined }}>
              {l.line}
            </div>
          ))}
        </div>

        <div style={{ display: "flex", justifyContent: "flex-end", gap: 8, marginTop: 12 }}>
          <button className="btn-secondary" onClick={onClose} disabled={running} title={running ? "Command still running" : "Close"}>
            Close
          </button>
        </div>
      </div>
    </div>
  );
}
