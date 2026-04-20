import { useState, useMemo } from "react";
import type { SlackDMConfig } from "../types";

function TrashIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M2 4h12M5 4V2.5A1.5 1.5 0 0 1 6.5 1h3A1.5 1.5 0 0 1 11 2.5V4m2 0v9a1.5 1.5 0 0 1-1.5 1.5h-7A1.5 1.5 0 0 1 3 13V4h10Z" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
    </svg>
  );
}

interface DMMessage {
  user: string;
  text: string;
  timestamp: string;
}

interface DMEntry {
  id: string;
  name?: string;
  userName?: string;
  isIm?: boolean;
  messages?: DMMessage[];
}

// Matches lines like: [2026-03-18 14:12 UTC] U09TCSM4FR6: message text here
const MESSAGE_LINE_RE = /^\[([^\]]+)\]\s+(.+?):\s+(.+)$/;

function parsePlainTextDMs(content: string): DMEntry[] | null {
  const lines = content.split("\n");
  const entries: DMEntry[] = [];
  let currentName: string | null = null;
  let currentMessages: DMMessage[] = [];

  const flush = () => {
    if (currentName !== null) {
      entries.push({ id: `conv-${entries.length}`, userName: currentName, messages: currentMessages });
    }
  };

  for (const line of lines) {
    if (line.startsWith("# DM with ")) {
      flush();
      currentName = line.slice("# DM with ".length).trim();
      currentMessages = [];
    } else if (currentName !== null) {
      const match = line.match(MESSAGE_LINE_RE);
      if (match) {
        currentMessages.push({ timestamp: match[1], user: match[2], text: match[3] });
      }
    }
  }
  flush();

  return entries.length > 0 ? entries : null;
}

function parseDMEntries(content: string): DMEntry[] | null {
  // Try JSON first (legacy / future structured format).
  try {
    const parsed = JSON.parse(content);
    if (Array.isArray(parsed) && parsed.length > 0 && "id" in parsed[0]) {
      return parsed as DMEntry[];
    }
  } catch { /* not JSON — try plain text */ }

  // Parse the plain-text format written by formatDMContent.
  return parsePlainTextDMs(content);
}

function entryLabel(entry: DMEntry): string {
  return entry.userName || entry.name || entry.id;
}

function DMConversations({ entries, onDeleteEntry }: { entries: DMEntry[]; onDeleteEntry?: (entryId: string) => void }) {
  return (
    <div className="dm-conversations">
      {entries.map((entry) => (
        <div key={entry.id} className="dm-conversation">
          <div className="dm-conversation-header">
            {entryLabel(entry)}
            {onDeleteEntry && (
              <button
                type="button"
                className="dm-entry-delete-btn"
                title={`Remove conversation with ${entryLabel(entry)}`}
                onClick={() => onDeleteEntry(entry.id)}
              >✕</button>
            )}
          </div>
          {entry.messages && entry.messages.length > 0 ? (
            <div className="dm-messages">
              {entry.messages.map((msg, i) => (
                <div key={i} className="dm-message">
                  <span className="dm-message-author">{msg.user}</span>
                  <span className="dm-message-time">{msg.timestamp}</span>
                  <div className="dm-message-text">{msg.text}</div>
                </div>
              ))}
            </div>
          ) : (
            <div className="dm-no-messages">No messages in time range</div>
          )}
        </div>
      ))}
    </div>
  );
}

interface Props {
  dm: SlackDMConfig;
  onDelete: (id: string) => void;
  onToggle: (id: string) => void;
  onDeleteEntry?: (dmConfigId: string, entryId: string) => void;
}

export default function SlackDMConfigCard({ dm, onDelete, onToggle, onDeleteEntry }: Props) {
  const [expanded, setExpanded] = useState(false);
  const isInactive = !dm.isActive;

  const label = dm.startFrom ? `DMs · ${dm.workspace} since ${dm.startFrom}` : `DMs · ${dm.workspace}`;

  const dmEntries = useMemo(() => dm.content ? parseDMEntries(dm.content) : null, [dm.content]);

  return (
    <div className={`issue-card${isInactive ? " issue-card--inactive" : ""}`}>
      <div
        className="issue-card-header"
        onClick={() => dm.content && setExpanded(!expanded)}
        style={{ cursor: dm.content ? "pointer" : "default" }}
      >
        <label
          className="item-active-toggle"
          title={isInactive ? "Inactive — click to activate" : "Active — click to deactivate"}
          onClick={(e) => { e.stopPropagation(); onToggle(dm.id); }}
        >
          <input type="checkbox" checked={dm.isActive} onChange={() => {}} />
          <span className={`item-active-indicator${dm.isActive ? " item-active-indicator--on" : ""}`} />
        </label>
        <span style={{ fontSize: 13, marginRight: 4, color: "var(--text-muted)" }}>✉</span>
        <span className={`issue-summary${isInactive ? " item--inactive" : ""}`}>{label}</span>
        <div className="issue-meta">
          {dm.content && (
            <span className={`expand-icon ${expanded ? "expanded" : ""}`}>▼</span>
          )}
          <button
            className="card-delete-btn"
            onClick={(e) => { e.stopPropagation(); onDelete(dm.id); }}
            title="Remove DM config"
          >
            <TrashIcon />
          </button>
        </div>
      </div>

      {expanded && dm.content && (
        <div className="issue-body">
          {dmEntries ? (
            <DMConversations
              entries={dmEntries}
              onDeleteEntry={onDeleteEntry ? (entryId) => onDeleteEntry(dm.id, entryId) : undefined}
            />
          ) : (
            <pre className="slack-thread-content">{dm.content}</pre>
          )}
        </div>
      )}
    </div>
  );
}
