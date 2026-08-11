import { useEffect } from "react";
import type { AppLogEntry } from "../types";

function prettyFields(fields?: string): { request?: string; rest: string } {
  if (!fields) return { rest: "" };
  try {
    const obj = JSON.parse(fields) as Record<string, unknown>;
    const request = typeof obj.request === "string" ? obj.request : undefined;
    const { request: _omit, ...rest } = obj;
    return { request, rest: JSON.stringify(rest, null, 2) };
  } catch {
    return { rest: fields };
  }
}

export function AppLogDetailModal({ entry, onClose }: { entry: AppLogEntry; onClose: () => void }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const { request, rest } = prettyFields(entry.fields);

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" role="dialog" aria-modal="true" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <span className={`settings-logs-trigger settings-logs-trigger--${entry.level}`}>{entry.level}</span>
          <span className="app-log-modal-caller">{entry.caller}</span>
        </div>
        <div className="modal-scroll-body">
          <p className="app-log-modal-msg">{entry.msg}</p>
          {entry.error && <p className="app-log-error-detail">{entry.error}</p>}
          {request && (
            <>
              <h4 className="app-log-modal-section">Request</h4>
              <pre className="app-log-modal-pre">{request}</pre>
            </>
          )}
          {rest && rest !== "{}" && (
            <>
              <h4 className="app-log-modal-section">Fields</h4>
              <pre className="app-log-modal-pre">{rest}</pre>
            </>
          )}
        </div>
        <div className="modal-footer">
          <button className="btn-secondary" onClick={onClose}>Close</button>
        </div>
      </div>
    </div>
  );
}
