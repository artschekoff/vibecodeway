import { X, TerminalSquare } from "lucide-react";
import { useTabContext } from "../contexts/TabContext";
import { useWaitingTerminals, clearWaiting, jumpToTerminal, readPrefs } from "../lib/terminalAttention";

// Global "an agent is waiting for you" stack, mounted once in App. Lists every
// terminal whose agent rang the bell while that terminal was NOT on screen;
// click jumps to it (tab + terminal focus, which clears the entry).
export function AgentWaitingToast() {
  const list = useWaitingTerminals();
  const tabs = useTabContext();
  const shown = list.filter((e) => !e.visible);
  if (!readPrefs().toast || shown.length === 0) return null;
  return (
    <div className="agent-toast-stack" role="status" aria-label="Agents waiting for input">
      {shown.slice(-4).map((e) => (
        <div key={e.sessionId} className="agent-toast">
          <button type="button" className="agent-toast__body" onClick={() => jumpToTerminal(e, tabs)}
            title="Jump to this terminal">
            <TerminalSquare size={13} />
            <span className="agent-toast__text">
              <span className="agent-toast__key">{e.contextKey || "Ask AI"}</span>
              <span className="agent-toast__tab"> · {e.tabLabel}</span>
              <span className="agent-toast__msg"> — {e.title}{e.body ? `: ${e.body}` : ""}</span>
            </span>
          </button>
          <button type="button" className="agent-toast__close" aria-label={`Dismiss ${e.contextKey || "Ask AI"}`}
            onClick={() => clearWaiting(e.sessionId)}><X size={12} /></button>
        </div>
      ))}
    </div>
  );
}
