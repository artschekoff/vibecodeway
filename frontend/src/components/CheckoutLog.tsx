import { useEffect, useState } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import CheckoutLogModal from "./CheckoutLogModal";
import type { CheckoutLogLine } from "../lib/useCheckout";

const isWailsContext = () => typeof (window as unknown as Record<string, unknown>)["go"] !== "undefined";

// The on-checkout log window, mounted once in App: every checkout (git worktree
// add, on-checkout scripts) emits checkout:start/log/done, and the user may be
// on any tab — or already switched into the editor — when it runs.
export function CheckoutLog() {
  const [open, setOpen] = useState(false);
  const [running, setRunning] = useState(false);
  const [logs, setLogs] = useState<CheckoutLogLine[]>([]);
  const [command, setCommand] = useState("");
  const [exitCode, setExitCode] = useState<number | null>(null);

  useEffect(() => {
    if (!isWailsContext()) return;
    const offStart = EventsOn("checkout:start", (p: { path?: string; cmd?: string }) => {
      setLogs([]); setExitCode(null); setCommand(p?.cmd ?? ""); setRunning(true); setOpen(true);
    });
    const offLog = EventsOn("checkout:log", (p: CheckoutLogLine) => { if (p) setLogs((prev) => [...prev, p]); });
    const offDone = EventsOn("checkout:done", (p: { exitCode?: number }) => { setRunning(false); setExitCode(p?.exitCode ?? 0); });
    return () => { offStart(); offLog(); offDone(); };
  }, []);

  return <CheckoutLogModal open={open} running={running} logs={logs} command={command} exitCode={exitCode} onClose={() => setOpen(false)} />;
}
