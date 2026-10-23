import { useState, useEffect, useRef } from "react";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";

const isWailsContext = () =>
  typeof (window as unknown as Record<string, unknown>)["go"] !== "undefined";

interface ProgressPayload {
  current: number;
  total: number;
  contextKey: string;
  step?: string; // provider being fetched right now: "jira" | "slack" | "vcs"; "done"/"error" = context finished
}

interface DonePayload {
  total: number;
  errors: number;
}

// One in-flight context: contexts refresh in PARALLEL (worker pool on the
// backend), so several rows can be live at once. lastAt is when its latest
// step event arrived — a row whose lastAt stops moving is the hang.
type ActiveRow = { step: string; since: number; lastAt: number };

type ToastState =
  | { phase: "idle" }
  | { phase: "running"; current: number; total: number }
  | { phase: "done"; total: number; errors: number };

// A row without events for this long is flagged as possibly stuck.
const STUCK_MS = 60_000;

export function BgRefreshToast() {
  const [state, setState] = useState<ToastState>({ phase: "idle" });
  const [active, setActive] = useState<Map<string, ActiveRow>>(new Map());
  const [expanded, setExpanded] = useState(false);
  // Ticks a re-render each second while running, so the per-row age counters move.
  const [, setTick] = useState(0);
  const dismissTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (!isWailsContext()) return;
    const offStart = EventsOn("bg-refresh:contexts:start", () => {
      if (dismissTimer.current) clearTimeout(dismissTimer.current);
      setActive(new Map());
      setState({ phase: "running", current: 0, total: 0 });
    });

    const offProgress = EventsOn("bg-refresh:contexts:progress", (payload: ProgressPayload) => {
      setState({ phase: "running", current: payload.current, total: payload.total });
      const step = payload.step ?? "";
      setActive((prev) => {
        const next = new Map(prev);
        if (step === "done" || step === "error") {
          next.delete(payload.contextKey);
        } else if (payload.contextKey) {
          const now = Date.now();
          const row = next.get(payload.contextKey);
          next.set(payload.contextKey, { step, since: row?.since ?? now, lastAt: now });
        }
        return next;
      });
    });

    const offDone = EventsOn("bg-refresh:contexts:done", (payload: DonePayload) => {
      setActive(new Map());
      setState({ phase: "done", total: payload.total, errors: payload.errors });
      dismissTimer.current = setTimeout(() => setState({ phase: "idle" }), 4000);
    });

    return () => {
      if (typeof offStart === "function") (offStart as () => void)(); else EventsOff("bg-refresh:contexts:start");
      if (typeof offProgress === "function") (offProgress as () => void)(); else EventsOff("bg-refresh:contexts:progress");
      if (typeof offDone === "function") (offDone as () => void)(); else EventsOff("bg-refresh:contexts:done");
      if (dismissTimer.current) clearTimeout(dismissTimer.current);
    };
  }, []);

  const running = state.phase === "running";
  useEffect(() => {
    if (!running) return;
    const h = setInterval(() => setTick((n) => n + 1), 1000);
    return () => clearInterval(h);
  }, [running]);

  if (state.phase === "idle") return null;

  if (state.phase === "running") {
    const pct = state.total > 0 ? Math.round((state.current / state.total) * 100) : 0;
    const rows = [...active.entries()].sort((a, b) => a[1].since - b[1].since);
    const now = Date.now();
    return (
      <div className="bg-refresh-toast" style={{ cursor: "pointer", pointerEvents: "auto" }} onClick={() => setExpanded((v) => !v)}
        title={expanded ? "Collapse" : "Show contexts refreshing right now"}>
        <div className="bg-refresh-toast__label">
          {state.total > 0
            ? `Updating contexts… ${state.current}/${state.total}`
            : "Updating contexts…"}
          <span className="bg-refresh-toast__key"> {expanded ? "▾" : `${rows.length} in flight ▸`}</span>
        </div>
        {expanded && rows.length > 0 && (
          <div style={{ display: "flex", flexDirection: "column", gap: 2, margin: "4px 0 6px", maxHeight: 180, overflowY: "auto" }}>
            {rows.map(([key, r]) => {
              const age = Math.round((now - r.lastAt) / 1000);
              const stuck = now - r.lastAt > STUCK_MS;
              return (
                <div key={key} style={{ display: "flex", alignItems: "baseline", gap: 6, fontSize: 11, minWidth: 0 }}>
                  <span style={{ fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{key}</span>
                  <span style={{ color: "var(--text-muted)", flexShrink: 0 }}>{r.step || "…"}</span>
                  <span style={{ marginLeft: "auto", flexShrink: 0, fontVariantNumeric: "tabular-nums",
                    color: stuck ? "var(--danger, #e5534b)" : "var(--text-muted)" }}>
                    {stuck ? `stuck? ${age}s` : `${age}s`}
                  </span>
                </div>
              );
            })}
          </div>
        )}
        <div className="bg-refresh-toast__bar">
          <div className="bg-refresh-toast__fill" style={{ width: state.total > 0 ? `${pct}%` : "0%" }} />
        </div>
      </div>
    );
  }

  const successful = state.total - state.errors;
  const msg =
    state.errors > 0
      ? `${successful}/${state.total} contexts updated, ${state.errors} error${state.errors !== 1 ? "s" : ""}`
      : `${state.total} context${state.total !== 1 ? "s" : ""} updated`;

  return (
    <div className="bg-refresh-toast bg-refresh-toast--done">
      ✓ {msg}
    </div>
  );
}
