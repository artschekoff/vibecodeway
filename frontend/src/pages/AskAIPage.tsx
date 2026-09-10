import { useState, useEffect, useRef } from "react";
import { Send, Loader2, Wrench, Eraser, FileStack } from "lucide-react";
import {
  AskAIChat,
  SummarizeAskChat,
  ListOrganizations,
  ListAgentTools,
  ResolveToolConfirmation,
} from "../../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import type { agent } from "../../wailsjs/go/models";
import type { Organization } from "../types";
import { ProseEditor } from "../components/ProseEditor";

type ProgressEvent = { phase: string; tool?: string; args?: string; result?: string; callId?: string };
type PendingConfirm = { tool: string; args: string; callId: string };

// A turn in the persistent, session-scoped chat log. "summary" entries replace
// the whole log after "Суммаризовать контекст" is used.
type LogEntry =
  | { role: "user"; content: string }
  | { role: "assistant"; content: string; steps: agent.Step[] }
  | { role: "summary"; content: string };

// toHistory converts the on-screen log into the {role, content} shape AskAIChat
// and SummarizeAskChat expect. A prior "summary" entry is sent as a system
// turn so the model treats it as ground truth about earlier context rather
// than something the user said.
function toHistory(entries: LogEntry[]): { role: string; content: string }[] {
  return entries.map((e) =>
    e.role === "summary"
      ? { role: "system", content: "Summary of the previous conversation:\n" + e.content }
      : { role: e.role, content: e.content }
  );
}

export default function AskAIPage() {
  const [question, setQuestion] = useState("");
  const [log, setLog] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [orgID, setOrgID] = useState("");
  const [progress, setProgress] = useState<ProgressEvent[]>([]);
  const [tools, setTools] = useState<{ name: string; description: string; write?: boolean }[]>([]);
  const [pendingConfirm, setPendingConfirm] = useState<PendingConfirm | null>(null);
  const bottomRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    ListOrganizations().then((list) => setOrgs(list ?? [])).catch(() => {});
  }, []);

  useEffect(() => {
    void ListAgentTools().then((t) => setTools(t ?? [])).catch(() => setTools([]));
  }, []);

  useEffect(() => {
    const off = EventsOn("ask:progress", (ev: ProgressEvent) => {
      if (ev.phase === "confirm" && ev.callId) {
        setPendingConfirm({ tool: ev.tool ?? "", args: ev.args ?? "", callId: ev.callId });
        return;
      }
      // A tool event for the same call clears the pending prompt (approved or declined).
      if (ev.phase === "tool") setPendingConfirm(null);
      setProgress((p) => [...p, ev]);
    });
    return () => {
      off();
      EventsOff("ask:progress");
    };
  }, []);

  // Auto-scroll to the bottom whenever the log grows or the live trace updates.
  useEffect(() => {
    bottomRef.current?.scrollIntoView?.({ block: "end" });
  }, [log, progress]);

  const ask = async () => {
    const q = question.trim();
    if (!q || loading) return;
    setLoading(true);
    setError("");
    setProgress([]);
    setPendingConfirm(null);
    const historyJSON = JSON.stringify(toHistory(log));
    setLog((l) => [...l, { role: "user", content: q }]);
    setQuestion("");
    try {
      const res = await AskAIChat(historyJSON, q, orgID);
      setLog((l) => [...l, { role: "assistant", content: res.content ?? "", steps: res.steps ?? [] }]);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  const clearContext = () => {
    if (loading || log.length === 0) return;
    setLog([]);
    setProgress([]);
    setError("");
  };

  const summarizeContext = async () => {
    if (loading || log.length === 0) return;
    setLoading(true);
    setError("");
    try {
      const historyJSON = JSON.stringify(toHistory(log));
      const content = await SummarizeAskChat(historyJSON);
      setLog([{ role: "summary", content }]);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    // Cmd/Ctrl+Enter submits.
    if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
      e.preventDefault();
      void ask();
    }
  };

  return (
    <div className="page" style={{ width: "100%", maxWidth: 860, margin: "0 auto", padding: 16 }}>
      <h1 style={{ fontSize: 18, marginBottom: 12 }}>Ask AI</h1>
      <p style={{ color: "var(--text-muted)", marginTop: 0, fontSize: 13 }}>
        Ask a question about your contexts. The agent searches contexts, reads relevant items, and follows dependencies to answer.
      </p>

      <div style={{ marginBottom: 10, display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
        <select
          value={orgID}
          onChange={(e) => setOrgID(e.target.value)}
          style={{ fontSize: 13, padding: "4px 8px", borderRadius: 6, border: "1px solid var(--border)" }}
        >
          <option value="">All organizations</option>
          {orgs.map((o) => (
            <option key={o.id} value={o.id}>{o.name}</option>
          ))}
        </select>
        <button
          className="btn btn-secondary"
          onClick={clearContext}
          disabled={loading || log.length === 0}
          style={{ fontSize: 12, display: "inline-flex", alignItems: "center", gap: 5 }}
        >
          <Eraser size={13} /> Очистить контекст
        </button>
        <button
          className="btn btn-secondary"
          onClick={() => void summarizeContext()}
          disabled={loading || log.length === 0}
          style={{ fontSize: 12, display: "inline-flex", alignItems: "center", gap: 5 }}
        >
          <FileStack size={13} /> Суммаризовать контекст
        </button>
      </div>

      {tools.length > 0 && (
        <details style={{ marginBottom: 16 }}>
          <summary style={{ cursor: "pointer", color: "var(--text-muted)" }}>
            What the assistant can do ({tools.length} tools)
          </summary>
          <div style={{ display: "flex", flexDirection: "column", gap: 8, marginTop: 8 }}>
            {tools.map((t) => (
              <div key={t.name} style={{ fontSize: 12 }}>
                <span style={{ fontFamily: "monospace", fontWeight: 600, color: "var(--text)" }}>{t.name}</span>
                {t.write && (
                  <span style={{ marginLeft: 6, fontSize: 10, fontWeight: 600, padding: "1px 5px", borderRadius: 4, border: "1px solid var(--warning, #d9a441)", color: "var(--warning, #d9a441)" }}>
                    WRITE
                  </span>
                )}
                <div style={{ color: "var(--text-muted)" }}>{t.description}</div>
              </div>
            ))}
          </div>
        </details>
      )}

      {pendingConfirm && (
        <div
          style={{
            marginBottom: 12, padding: 12, border: "1px solid var(--warning, #d9a441)",
            borderRadius: 8, background: "var(--bg-subtle, rgba(217,164,65,0.08))",
          }}
        >
          <div style={{ fontSize: 13, marginBottom: 6 }}>
            The assistant wants to run <strong style={{ fontFamily: "monospace" }}>{pendingConfirm.tool}</strong>, which changes data:
          </div>
          <pre style={{ fontSize: 12, background: "var(--bg, rgba(0,0,0,0.04))", padding: 8, borderRadius: 6, overflowX: "auto", margin: "0 0 8px" }}>
            {pendingConfirm.args || "{}"}
          </pre>
          <div style={{ display: "flex", gap: 8 }}>
            <button
              onClick={() => { ResolveToolConfirmation(pendingConfirm.callId, true); setPendingConfirm(null); }}
              style={{ fontSize: 13, padding: "5px 12px", borderRadius: 6, border: "1px solid var(--border)", cursor: "pointer" }}
            >
              Approve
            </button>
            <button
              onClick={() => { ResolveToolConfirmation(pendingConfirm.callId, false); setPendingConfirm(null); }}
              style={{ fontSize: 13, padding: "5px 12px", borderRadius: 6, border: "1px solid var(--border)", cursor: "pointer" }}
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        {log.map((entry, i) => {
          if (entry.role === "user") {
            return (
              <div key={i} style={{ alignSelf: "flex-end", maxWidth: "85%" }}>
                <div
                  style={{
                    whiteSpace: "pre-wrap", fontSize: 14, lineHeight: 1.5, padding: "10px 14px",
                    borderRadius: 10, background: "var(--accent)", color: "var(--accent-text)",
                  }}
                >
                  {entry.content}
                </div>
              </div>
            );
          }
          if (entry.role === "summary") {
            return (
              <div
                key={i}
                style={{
                  fontSize: 13, padding: 12, borderRadius: 8,
                  border: "1px dashed var(--border)", background: "var(--surface2)", color: "var(--text-muted)",
                }}
              >
                <div style={{ fontWeight: 600, marginBottom: 4, color: "var(--text)" }}>Контекст сжат</div>
                <div style={{ whiteSpace: "pre-wrap" }}>{entry.content}</div>
              </div>
            );
          }
          // assistant
          return (
            <div key={i}>
              <div
                style={{ whiteSpace: "pre-wrap", fontSize: 14, lineHeight: 1.55, padding: 14, border: "1px solid var(--border)", borderRadius: 8 }}
              >
                {entry.content || "(no answer)"}
              </div>
              {entry.steps.length > 0 && (
                <details style={{ marginTop: 8 }}>
                  <summary style={{ cursor: "pointer", color: "var(--text-muted)", fontSize: 13, display: "flex", alignItems: "center", gap: 6 }}>
                    <Wrench size={13} /> {entry.steps.length} tool call{entry.steps.length !== 1 ? "s" : ""}
                  </summary>
                  <div style={{ marginTop: 8, display: "flex", flexDirection: "column", gap: 8 }}>
                    {entry.steps.map((s, j) => (
                      <div key={j} style={{ fontSize: 12, fontFamily: "monospace", background: "var(--bg-subtle, rgba(0,0,0,0.04))", padding: 8, borderRadius: 6 }}>
                        <div style={{ fontWeight: 600 }}>{s.tool}({s.args})</div>
                        <div style={{ color: "var(--text-muted)", whiteSpace: "pre-wrap", marginTop: 4 }}>
                          {s.result.length > 600 ? s.result.slice(0, 600) + "…" : s.result}
                        </div>
                      </div>
                    ))}
                  </div>
                </details>
              )}
            </div>
          );
        })}

        {loading && progress.length > 0 && (
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            {progress.map((ev, i) => (
              <div key={i} style={{ fontSize: 12, fontFamily: "monospace", color: "var(--text-muted)" }}>
                {ev.phase === "tool" ? `→ ${ev.tool}(${ev.args ?? ""})` : "· thinking…"}
              </div>
            ))}
          </div>
        )}

        {error && (
          <div className="detail-section-empty" style={{ color: "var(--danger, #c0392b)" }}>
            {error}
          </div>
        )}

        <div ref={bottomRef} />
      </div>

      <div style={{ display: "flex", flexDirection: "column", gap: 8, marginTop: 16 }}>
        <ProseEditor
          value={question}
          onChange={(e) => setQuestion(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder="e.g. Which contexts depend on the payments refactor, and what's blocking them?"
          rows={3}
          style={{ width: "100%", resize: "vertical", padding: 10 }}
        />
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <button className="btn btn-primary" onClick={() => void ask()} disabled={loading || !question.trim()}>
            {loading ? <Loader2 size={14} className="spin" /> : <Send size={14} />}
            {loading ? " Thinking…" : " Ask"}
          </button>
          <span style={{ color: "var(--text-muted)", fontSize: 12 }}>⌘/Ctrl + Enter</span>
        </div>
      </div>
    </div>
  );
}
