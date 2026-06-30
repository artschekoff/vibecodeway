import { useState, useEffect, useRef } from "react";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";

const isWailsContext = () =>
  typeof (window as unknown as Record<string, unknown>)["go"] !== "undefined";

interface ProgressPayload {
  current: number;
  total: number;
  contextKey: string;
}

interface DonePayload {
  total: number;
  errors: number;
}

type ToastState =
  | { phase: "idle" }
  | { phase: "running"; current: number; total: number; contextKey: string }
  | { phase: "done"; total: number; errors: number };

export function BgRefreshToast() {
  const [state, setState] = useState<ToastState>({ phase: "idle" });
  const dismissTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (!isWailsContext()) return;
    const offStart = EventsOn("bg-refresh:contexts:start", () => {
      if (dismissTimer.current) clearTimeout(dismissTimer.current);
      setState({ phase: "running", current: 0, total: 0, contextKey: "" });
    });

    const offProgress = EventsOn("bg-refresh:contexts:progress", (payload: ProgressPayload) => {
      setState({
        phase: "running",
        current: payload.current,
        total: payload.total,
        contextKey: payload.contextKey,
      });
    });

    const offDone = EventsOn("bg-refresh:contexts:done", (payload: DonePayload) => {
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

  if (state.phase === "idle") return null;

  if (state.phase === "running") {
    const pct = state.total > 0 ? Math.round((state.current / state.total) * 100) : 0;
    return (
      <div className="bg-refresh-toast">
        <div className="bg-refresh-toast__label">
          {state.total > 0
            ? `Updating contexts… ${state.current}/${state.total}`
            : "Updating contexts…"}
          {state.contextKey && (
            <span className="bg-refresh-toast__key"> {state.contextKey}</span>
          )}
        </div>
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
