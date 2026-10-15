import { useState, useEffect, useRef } from "react";
import { Send, Loader2, Wrench, Eraser, FileStack, History, Trash2 } from "lucide-react";
import {
  AskAIChat,
  SummarizeAskChat,
  ListOrganizations,
  ListAgentTools,
  ResolveToolConfirmation,
  SaveAskChat,
  ListAskChats,
  GetAskChat,
  DeleteAskChat,
} from "../../wailsjs/go/main/App";
import { EventsOn, EventsOff, BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import type { agent, store } from "../../wailsjs/go/models";
import type { Organization } from "../types";
import { ProseEditor } from "../components/ProseEditor";
import { MarkdownBody } from "../components/MarkdownBody";
import { useTabContextOptional } from "../contexts/TabContext";

type ProgressEvent = { phase: string; tool?: string; args?: string; result?: string; callId?: string };

// Height of one line in the live-progress odometer ticker.
const TICKER_LINE = 18;
type PendingConfirm = { tool: string; args: string; callId: string };

// A turn in the persistent, session-scoped chat log. "summary" entries replace
// the whole log after "Summarize context" is used.
type LogEntry =
  | { role: "user"; content: string }
  | { role: "assistant"; content: string; steps: agent.Step[] }
  | { role: "summary"; content: string };

// Tool results replayed into history are capped so a chatty turn can't blow
// the model's context: per result, and in total across the whole log (newest
// turns keep theirs first; older ones degrade to text-only).
const HISTORY_RESULT_CHARS = 8_000;
const HISTORY_TOOL_BUDGET_CHARS = 80_000;

// toHistory converts the on-screen log into the {role, content} shape AskAIChat
// and SummarizeAskChat expect. A prior "summary" entry is sent as a system
// turn so the model treats it as ground truth about earlier context rather
// than something the user said. Tool results an assistant turn gathered are
// replayed as a system turn right before that answer — otherwise the model
// forgets everything it read (Jira comments, PR bodies) on the next question.
export function toHistory(entries: LogEntry[]): { role: string; content: string }[] {
  // Decide per entry (newest first) whether its tool results fit the budget.
  let budget = HISTORY_TOOL_BUDGET_CHARS;
  const toolBlocks = new Map<number, string>();
  for (let i = entries.length - 1; i >= 0; i--) {
    const e = entries[i];
    if (e.role !== "assistant" || e.steps.length === 0) continue;
    const block = e.steps
      .map((s) => `## ${s.tool} ${s.args}\n${s.result.length > HISTORY_RESULT_CHARS ? s.result.slice(0, HISTORY_RESULT_CHARS) + "…[truncated]" : s.result}`)
      .join("\n\n");
    if (block.length > budget) break;
    budget -= block.length;
    toolBlocks.set(i, block);
  }

  const out: { role: string; content: string }[] = [];
  entries.forEach((e, i) => {
    if (e.role === "summary") {
      out.push({ role: "system", content: "Summary of the previous conversation:\n" + e.content });
      return;
    }
    const block = toolBlocks.get(i);
    if (block) out.push({ role: "system", content: "Tool results the assistant retrieved for its next reply (treat as already-known facts):\n\n" + block });
    out.push({ role: e.role, content: e.content });
  });
  return out;
}

// Chat log survives tab close / app restart. One shared log (the page is a
// singleton), stored whole — summarize already keeps it bounded. The same log
// is also persisted to the DB under CHAT_ID_KEY (see SaveAskChat) so past
// chats can be reopened from the History picker and dumped via the CLI.
const LOG_KEY = "vibecodeway:askai:log";
const CHAT_ID_KEY = "vibecodeway:askai:chatId";

function loadLog(): LogEntry[] {
  try {
    return JSON.parse(localStorage.getItem(LOG_KEY) ?? "[]") as LogEntry[];
  } catch {
    return [];
  }
}

function newChatId(): string {
  return typeof crypto !== "undefined" && "randomUUID" in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function loadChatId(): string {
  try {
    const id = localStorage.getItem(CHAT_ID_KEY);
    if (id) return id;
  } catch { /* fall through */ }
  return newChatId();
}

// Title shown in the History picker: the first user question (or the summary).
function chatTitle(entries: LogEntry[]): string {
  const first = entries.find((e) => e.role === "user") ?? entries[0];
  return (first?.content ?? "").replace(/\s+/g, " ").trim();
}

export default function AskAIPage() {
  const [question, setQuestion] = useState("");
  const [log, setLog] = useState<LogEntry[]>(loadLog);
  const [chatId, setChatId] = useState<string>(loadChatId);
  const [chats, setChats] = useState<store.AskChat[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [orgID, setOrgID] = useState("");
  const [progress, setProgress] = useState<ProgressEvent[]>([]);
  const [tools, setTools] = useState<{ name: string; description: string; write?: boolean }[]>([]);
  const [pendingConfirm, setPendingConfirm] = useState<PendingConfirm | null>(null);
  const bottomRef = useRef<HTMLDivElement | null>(null);
  const tabs = useTabContextOptional();

  // Links in answers: context://<id> opens the context in a new app tab
  // (the link text is the context key, used as the tab title); normal URLs
  // open in the system browser.
  const onAnswerLink = (href: string, label: string) => {
    if (href.startsWith("context://")) {
      const id = href.slice("context://".length);
      tabs?.openNewContextTab(id, label || id);
      return;
    }
    if (/^https?:\/\//i.test(href)) BrowserOpenURL(href);
  };

  useEffect(() => {
    try {
      localStorage.setItem(LOG_KEY, JSON.stringify(log));
    } catch {
      // Quota (huge tool results in steps): keep the text, drop the steps.
      try {
        localStorage.setItem(LOG_KEY, JSON.stringify(log.map((e) => (e.role === "assistant" ? { ...e, steps: [] } : e))));
      } catch { /* give up; log lives only in memory this session */ }
    }
  }, [log]);

  // Persist the active chat to the DB on every change (fire-and-forget).
  useEffect(() => {
    try { localStorage.setItem(CHAT_ID_KEY, chatId); } catch { /* ignore */ }
    if (log.length === 0) return;
    void SaveAskChat(chatId, chatTitle(log), JSON.stringify(log)).catch(() => {});
  }, [chatId, log]);

  const refreshChats = () => {
    void ListAskChats().then((list) => setChats(list ?? [])).catch(() => {});
  };

  const openChat = async (id: string) => {
    if (!id || id === chatId || loading) return;
    try {
      const chat = await GetAskChat(id);
      setLog(JSON.parse(chat.logJson || "[]") as LogEntry[]);
      setChatId(id);
      setProgress([]);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  };

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

  // Auto-scroll only when the log itself grows — progress ticks animate inside
  // the fixed-height ticker, so they must not yank the user away from where
  // they scrolled to.
  useEffect(() => {
    bottomRef.current?.scrollIntoView?.({ block: "end" });
  }, [log]);

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

  // Deletes the open chat from the DB and starts a fresh one.
  const deleteChat = async () => {
    if (loading || log.length === 0) return;
    try {
      await DeleteAskChat(chatId);
      setLog([]);
      setChatId(newChatId());
      setProgress([]);
      setError("");
      refreshChats();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  };

  // Starts a fresh chat; the previous one stays in History (DB).
  const clearContext = () => {
    if (loading || log.length === 0) return;
    setLog([]);
    setChatId(newChatId());
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
          <Eraser size={13} /> Clear context
        </button>
        <button
          className="btn btn-secondary"
          onClick={() => void summarizeContext()}
          disabled={loading || log.length === 0}
          style={{ fontSize: 12, display: "inline-flex", alignItems: "center", gap: 5 }}
        >
          <FileStack size={13} /> Summarize context
        </button>
        <label style={{ display: "inline-flex", alignItems: "center", gap: 5, fontSize: 12, color: "var(--text-muted)" }}>
          <History size={13} />
          <select
            aria-label="Chat history"
            value=""
            onFocus={refreshChats}
            onChange={(e) => void openChat(e.target.value)}
            disabled={loading}
            style={{ fontSize: 12, padding: "4px 8px", borderRadius: 6, border: "1px solid var(--border)", maxWidth: 320 }}
          >
            <option value="">History…</option>
            {chats.filter((c) => c.id !== chatId).map((c) => (
              <option key={c.id} value={c.id}>
                {new Date(c.updatedAt).toLocaleString(undefined, { dateStyle: "short", timeStyle: "short" })} — {c.title || "(untitled)"}
              </option>
            ))}
          </select>
        </label>
        <button
          className="btn btn-secondary"
          onClick={() => void deleteChat()}
          disabled={loading || log.length === 0}
          title="Delete this chat from history"
          aria-label="Delete chat"
          style={{ fontSize: 12, display: "inline-flex", alignItems: "center", gap: 5, color: "var(--danger)" }}
        >
          <Trash2 size={13} />
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

      {/* The log owns the scroll: the page never grows past the viewport, and both
          ends of a long dialog stay reachable. */}
      <div style={{ flex: 1, minHeight: 0, overflowY: "auto", display: "flex", flexDirection: "column", gap: 12, paddingRight: 4 }}>
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
                <div style={{ fontWeight: 600, marginBottom: 4, color: "var(--text)" }}>Context summarized</div>
                <div style={{ whiteSpace: "pre-wrap" }}>{entry.content}</div>
              </div>
            );
          }
          // assistant
          return (
            <div key={i}>
              <div
                style={{ fontSize: 14, lineHeight: 1.55, padding: 14, border: "1px solid var(--border)", borderRadius: 8 }}
              >
                <MarkdownBody text={entry.content || "(no answer)"} onLinkClick={onAnswerLink} />
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
          /* Odometer ticker: a fixed 3-line window; new status lines roll up
             smoothly via a translateY transition instead of growing the page. */
          <div
            style={{
              height: TICKER_LINE * 3, overflow: "hidden", flexShrink: 0,
              WebkitMaskImage: "linear-gradient(to bottom, transparent 0, #000 30%)",
              maskImage: "linear-gradient(to bottom, transparent 0, #000 30%)",
            }}
          >
            <div
              style={{
                transform: `translateY(${-Math.max(0, progress.length - 3) * TICKER_LINE}px)`,
                transition: "transform 0.25s ease",
              }}
            >
              {progress.map((ev, i) => (
                <div key={i} style={{ height: TICKER_LINE, lineHeight: `${TICKER_LINE}px`, fontSize: 12, fontFamily: "monospace", color: "var(--text-muted)", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                  {ev.phase === "tool" ? `→ ${ev.tool}(${ev.args ?? ""})` : "· thinking…"}
                </div>
              ))}
            </div>
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
