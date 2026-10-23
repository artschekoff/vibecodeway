import { useCallback, useEffect, useState } from "react";
import { CheckCircle2, AlertTriangle, XCircle, Info } from "lucide-react";
import { RunReadinessChecks, ResolveReadiness } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

export type ReadinessCheck = {
  id: string; title: string; detail: string; status: "ok" | "warn" | "fail" | "info";
  required: boolean; action?: string; actionLabel?: string;
};

// Settings deep link: the modal stores the tab, Settings reads it on mount.
export const SETTINGS_TAB_KEY = "vibecodeway:settings:tab";
// Startup gate: shown on first launch and whenever a required check fails.
export const READINESS_SEEN_KEY = "vibecodeway:readiness:seen";
export const OPEN_READINESS_EVENT = "vibecodeway:open-readiness";
const RECHECK_MS = 4000;

const ICON = {
  ok: <CheckCircle2 size={15} className="ready-icon ready-icon--ok" />,
  warn: <AlertTriangle size={15} className="ready-icon ready-icon--warn" />,
  fail: <XCircle size={15} className="ready-icon ready-icon--fail" />,
  info: <Info size={15} className="ready-icon ready-icon--info" />,
};

/**
 * Setup checklist: permissions, credentials, tools and optional extras with a
 * fix button per row. Rows come from Go (RunReadinessChecks); fix actions go
 * back to Go except "app-settings:<tab>", which opens the in-app Settings tab.
 */
export function ReadinessModal({ onClose, openSettings }: { onClose: () => void; openSettings: () => void }) {
  const [checks, setChecks] = useState<ReadinessCheck[] | null>(null);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      setChecks(((await RunReadinessChecks()) as unknown as ReadinessCheck[]) ?? []);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setChecks([]);
    }
  }, []);
  useEffect(() => { void load(); }, [load]);
  // Re-check by itself: every few seconds while open, and the moment the app
  // regains focus (back from System Settings / a terminal). The Accessibility
  // fix also polls macOS in the background and reports when granted.
  useEffect(() => {
    const timer = window.setInterval(() => { void load(); }, RECHECK_MS);
    const onFocus = () => { void load(); };
    window.addEventListener("focus", onFocus);
    document.addEventListener("visibilitychange", onFocus);
    let off: (() => void) | undefined;
    try { off = EventsOn("accessibility:granted", onFocus); } catch { /* runtime absent */ }
    return () => {
      window.clearInterval(timer);
      window.removeEventListener("focus", onFocus);
      document.removeEventListener("visibilitychange", onFocus);
      off?.();
    };
  }, [load]);

  const resolve = async (c: ReadinessCheck) => {
    if (!c.action) return;
    const [kind, arg] = c.action.split(/:(.*)/s);
    if (kind === "app-settings") {
      try { localStorage.setItem(SETTINGS_TAB_KEY, arg ?? ""); } catch { /* ignore */ }
      openSettings();
      onClose();
      return;
    }
    try { await ResolveReadiness(c.action); } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
  };

  const failing = (checks ?? []).filter((c) => c.required && c.status === "fail").length;
  return (
    <div className="modal-overlay" role="dialog" aria-modal="true" aria-label="Setup check" style={{ zIndex: 250 }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <div className="modal ready-modal">
        <div className="ready-head">
          <h2>Setup check</h2>
          <span className="ready-summary">
            {checks === null ? "Checking…" : failing > 0 ? `${failing} required item${failing > 1 ? "s" : ""} missing` : "All required items are in place"}
          </span>
        </div>
        {error && <div className="board-error">{error}</div>}
        <div className="ready-list">
          {(checks ?? []).map((c) => (
            <div key={c.id} className={`ready-row ready-row--${c.status}`} data-status={c.status}>
              {ICON[c.status]}
              <div className="ready-text">
                <div className="ready-title">{c.title}{c.required && <span className="ready-req">required</span>}</div>
                <div className="ready-detail">{c.detail}</div>
              </div>
              {c.action && c.actionLabel && (
                <button type="button" className={c.status === "fail" ? "btn-primary" : "btn-secondary"} style={{ fontSize: 12, whiteSpace: "nowrap" }}
                  onClick={() => void resolve(c)}>{c.actionLabel}</button>
              )}
            </div>
          ))}
        </div>
        <div className="modal-footer">
          <span style={{ flex: 1, fontSize: 11, color: "var(--text-muted)" }}>Checks refresh automatically. Reopen anytime from the ☰ menu → Setup check.</span>
          <button type="button" className="btn-primary" onClick={onClose}>Close</button>
        </div>
      </div>
    </div>
  );
}
