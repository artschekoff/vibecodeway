import { useState } from "react";

function CopyIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
      <rect x="5" y="5" width="9" height="9" rx="1.5" stroke="currentColor" strokeWidth="1.5"/>
      <path d="M11 5V3.5A1.5 1.5 0 0 0 9.5 2h-6A1.5 1.5 0 0 0 2 3.5v6A1.5 1.5 0 0 0 3.5 11H5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
    </svg>
  );
}

function TrashIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M2 4h12M5 4V2.5A1.5 1.5 0 0 1 6.5 1h3A1.5 1.5 0 0 1 11 2.5V4m2 0v9a1.5 1.5 0 0 1-1.5 1.5h-7A1.5 1.5 0 0 1 3 13V4h10Z" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
    </svg>
  );
}

interface SlackChannel {
  id: string;
  workspace: string;
  channelId: string;
  channelName: string;
  createdAt: string;
}

interface SlackThread {
  id: string;
  workspace: string;
  channelId: string;
  channelName: string;
  permalink: string;
  searchQuery: string;
  content: string;
  createdAt: string;
}

interface Props {
  channel: SlackChannel;
  threads: SlackThread[];
  onDeleteChannel?: (id: string) => void;
  onDeleteThread?: (id: string) => void;
}

function ThreadRow({ thread, onDelete }: { thread: SlackThread; onDelete?: (id: string) => void }) {
  const [expanded, setExpanded] = useState(false);

  const preview = thread.content
    ? thread.content.slice(0, 120).replace(/\n+/g, " ").trim() + (thread.content.length > 120 ? "…" : "")
    : "(no content — run Refresh to load)";

  const handleDelete = (e: React.MouseEvent) => {
    e.stopPropagation();
    onDelete?.(thread.id);
  };

  return (
    <div className="slack-thread-row">
      <div
        className={`slack-thread-row-header${expanded ? " slack-thread-row-header--open" : ""}`}
        onClick={() => setExpanded(!expanded)}
      >
        <span className="slack-tree-connector">└</span>
        <span className="slack-thread-row-preview">{preview}</span>
        <div className="slack-thread-row-meta">
          {thread.permalink && (
            <a
              href={thread.permalink}
              target="_blank"
              rel="noreferrer"
              className="slack-thread-link"
              onClick={(e) => e.stopPropagation()}
            >
              ↗
            </a>
          )}
          {thread.content && (
            <span className={`expand-icon ${expanded ? "expanded" : ""}`}>▼</span>
          )}
          {onDelete && (
            <button
              className="card-delete-btn card-delete-btn--sm"
              onClick={handleDelete}
              title="Remove this thread"
            >
              <TrashIcon />
            </button>
          )}
        </div>
      </div>

      {expanded && thread.content && (
        <div className="slack-thread-row-body">
          <pre className="slack-thread-content">{thread.content}</pre>
        </div>
      )}
    </div>
  );
}

export default function SlackChannelCard({ channel, threads, onDeleteChannel, onDeleteThread }: Props) {
  const [expanded, setExpanded] = useState(false);
  const [copied, setCopied] = useState(false);

  const copyName = (e: React.MouseEvent) => {
    e.stopPropagation();
    navigator.clipboard.writeText(channel.channelName || channel.channelId).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  };

  const handleDeleteChannel = (e: React.MouseEvent) => {
    e.stopPropagation();
    onDeleteChannel?.(channel.id);
  };

  return (
    <div className="issue-card">
      <div
        className="issue-card-header"
        onClick={() => setExpanded(!expanded)}
      >
        <span className="slack-thread-hash">#</span>
        <span className="issue-summary">{channel.channelName || channel.channelId}</span>
        <button className="copy-btn" onClick={copyName} title="Copy channel name">
          {copied ? "✓" : <CopyIcon />}
        </button>
        <div className="issue-meta">
          <span className="channel-result-workspace">{channel.workspace}</span>
          {threads.length > 0 ? (
            <span className="status-badge">
              {threads.length} thread{threads.length !== 1 ? "s" : ""}
            </span>
          ) : (
            <span className="status-badge" style={{ color: "var(--text-muted)", borderColor: "transparent", background: "transparent" }}>
              No threads yet
            </span>
          )}
          {threads.length > 0 && (
            <span className={`expand-icon ${expanded ? "expanded" : ""}`}>▼</span>
          )}
          {onDeleteChannel && (
            <button
              className="card-delete-btn"
              onClick={handleDeleteChannel}
              title="Remove this channel (and all its threads)"
            >
              <TrashIcon />
            </button>
          )}
        </div>
      </div>

      {expanded && threads.length > 0 && (
        <div className="issue-body slack-channel-threads">
          {threads.map((t) => (
            <ThreadRow key={t.id} thread={t} onDelete={onDeleteThread} />
          ))}
        </div>
      )}
    </div>
  );
}
