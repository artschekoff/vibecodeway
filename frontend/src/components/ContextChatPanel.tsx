import { useEffect, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import {
  ChatWithContext,
  ListContextChat,
  ApplyChatSuggestion,
} from "../../wailsjs/go/main/App";
import type { ChatMessage, ConsolidatedLog } from "../types";

interface Props {
  contextId: string;
  onConsolidatedUpdated?: (log: ConsolidatedLog) => void;
}

export default function ContextChatPanel({ contextId, onConsolidatedUpdated }: Props) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState("");
  const listRef = useRef<HTMLDivElement>(null);

  const loadMessages = async () => {
    try {
      const list = await ListContextChat(contextId);
      setMessages((list ?? []) as unknown as ChatMessage[]);
    } catch (e) {
      setError(String(e));
    }
  };

  useEffect(() => {
    void loadMessages();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [contextId]);

  useEffect(() => {
    listRef.current?.scrollTo({ top: listRef.current.scrollHeight });
  }, [messages]);

  const handleSend = async () => {
    const text = draft.trim();
    if (!text) return;
    setSending(true);
    setError("");
    setDraft("");
    try {
      await ChatWithContext(contextId, text);
      await loadMessages();
    } catch (e) {
      setError(String(e));
      setDraft(text);
    } finally {
      setSending(false);
    }
  };

  const handleApply = async (messageId: string) => {
    setError("");
    try {
      const updated = await ApplyChatSuggestion(messageId);
      onConsolidatedUpdated?.(updated);
      await loadMessages();
    } catch (e) {
      setError(String(e));
    }
  };

  return (
    <div className="chat-panel">
      {error && <div className="error-banner">{error}</div>}

      <div ref={listRef} className="chat-messages">
        {messages.length === 0 && (
          <div className="chat-empty">
            AI chat about the current context. The AI only sees the consolidated log —
            initialize it first if you haven't yet.
          </div>
        )}
        {messages.map((m) => (
          <div key={m.id} className={`chat-message chat-message--${m.role}`}>
            <div className="chat-message-role">{m.role === "user" ? "You" : "AI"}</div>
            <div className="chat-message-body">
              <ReactMarkdown>{m.content}</ReactMarkdown>
            </div>
            {m.proposedConsolidatedMd && (
              <div className="chat-proposal">
                <div className="chat-proposal-header">
                  <strong>Proposed consolidated log</strong>
                  {m.appliedAt ? (
                    <span className="chat-proposal-applied">applied</span>
                  ) : (
                    <button
                      className="btn-primary"
                      onClick={() => void handleApply(m.id)}
                      style={{ fontSize: 12, padding: "2px 10px" }}
                    >
                      Apply
                    </button>
                  )}
                </div>
                <div className="chat-proposal-body">
                  <ReactMarkdown>{m.proposedConsolidatedMd}</ReactMarkdown>
                </div>
              </div>
            )}
          </div>
        ))}
      </div>

      <div className="chat-input-row">
        <textarea
          className="chat-input"
          placeholder="Write a message…"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
              e.preventDefault();
              void handleSend();
            }
          }}
          rows={3}
          disabled={sending}
        />
        <button
          className="btn-primary"
          onClick={() => void handleSend()}
          disabled={sending || !draft.trim()}
          style={{ alignSelf: "flex-end" }}
        >
          {sending ? "…" : "Send"}
        </button>
      </div>
    </div>
  );
}
