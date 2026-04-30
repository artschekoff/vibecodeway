import { Check, ChevronDown, ExternalLink } from "lucide-react";
import { useState } from "react";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import ConfirmModal from "./ConfirmModal";

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

function ActiveToggle({ isActive, onToggle }: { isActive: boolean; onToggle: (e: React.MouseEvent) => void }) {
  return (
    <label className="item-active-toggle" onClick={(e) => e.stopPropagation()} title={isActive ? "Active — click to deactivate" : "Inactive — click to activate"}>
      <input type="checkbox" checked={isActive} onChange={() => {}} onClick={onToggle} />
      <span className={`item-active-indicator${isActive ? " item-active-indicator--on" : ""}`} />
    </label>
  );
}

interface SlackChannel {
  id: string;
  workspace: string;
  channelId: string;
  channelName: string;
  createdAt: string;
  isActive: boolean;
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
  isActive: boolean;
  viewedAt?: string;
}

interface Props {
  channel: SlackChannel;
  threads: SlackThread[];
  onDeleteChannel?: (id: string) => void;
  onDeleteThread?: (id: string) => void;
  onToggleChannel?: (id: string) => void;
  onToggleThread?: (id: string) => void;
  onMarkThreadViewed?: (id: string) => void;
}

function ThreadRow({ thread, onDelete, onToggle, onMarkViewed }: { thread: SlackThread; onDelete?: (id: string) => void; onToggle?: (id: string) => void; onMarkViewed?: (id: string) => void }) {
  const [expanded, setExpanded] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const isInactive = !thread.isActive;
  const isUnread = !thread.viewedAt;

  const preview = thread.content
    ? thread.content.slice(0, 120).replace(/\n+/g, " ").trim() + (thread.content.length > 120 ? "…" : "")
    : "(no content — run Refresh to load)";

  const handleDelete = (e: React.MouseEvent) => {
    e.stopPropagation();
    setConfirmDelete(true);
  };

  return (
    <div className={`slack-thread-row${isInactive ? " slack-thread-row--inactive" : ""}${isUnread ? " item--unread" : ""}`}>
      <div
        className={`slack-thread-row-header${expanded ? " slack-thread-row-header--open" : ""}`}
        onClick={() => setExpanded(!expanded)}
      >
        <span className="slack-tree-connector">└</span>
        {onToggle && (
          <ActiveToggle isActive={thread.isActive} onToggle={(e) => { e.stopPropagation(); onToggle(thread.id); }} />
        )}
        <span className={`slack-thread-row-preview${isInactive ? " item--inactive" : ""}`}>{preview}</span>
        <div className="slack-thread-row-meta">
          {thread.permalink && (
            <button
              className="copy-btn"
              onClick={(e) => { e.stopPropagation(); BrowserOpenURL(thread.permalink); }}
              title="Open thread in Slack"
            >
              <ExternalLink size={12} />
            </button>
          )}
          {isUnread && onMarkViewed && (
            <button className="mark-viewed-btn" onClick={(e) => { e.stopPropagation(); onMarkViewed(thread.id); }} title="Mark as viewed"><Check size={13} /></button>
          )}
          {thread.content && (
            <ChevronDown size={14} className={`expand-icon${expanded ? " expanded" : ""}`} />
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

      {confirmDelete && (
        <ConfirmModal
          message="Delete this thread?"
          onConfirm={() => { setConfirmDelete(false); onDelete?.(thread.id); }}
          onCancel={() => setConfirmDelete(false)}
        />
      )}
    </div>
  );
}

export default function SlackChannelCard({ channel, threads, onDeleteChannel, onDeleteThread, onToggleChannel, onToggleThread, onMarkThreadViewed }: Props) {
  const [expanded, setExpanded] = useState(false);
  const [copied, setCopied] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const isInactive = !channel.isActive;
  const hasUnreadThreads = threads.some((t) => !t.viewedAt);
  const slackChannelLink = channel.workspace && channel.channelId
    ? `https://app.slack.com/client/${channel.workspace}/${channel.channelId}`
    : null;

  const copyName = (e: React.MouseEvent) => {
    e.stopPropagation();
    navigator.clipboard.writeText(channel.channelName || channel.channelId).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  };

  const handleDeleteChannel = (e: React.MouseEvent) => {
    e.stopPropagation();
    setConfirmDelete(true);
  };

  return (
    <div className={`issue-card${isInactive ? " issue-card--inactive" : ""}${hasUnreadThreads ? " item--unread-parent" : ""}`}>
      <div
        className="issue-card-header"
        onClick={() => setExpanded(!expanded)}
      >
        {onToggleChannel && (
          <ActiveToggle isActive={channel.isActive} onToggle={(e) => { e.stopPropagation(); onToggleChannel(channel.id); }} />
        )}
        <span className="slack-thread-hash">#</span>
        <span className={`issue-summary${isInactive ? " item--inactive" : ""}`}>{channel.channelName || channel.channelId}</span>
        <button className="copy-btn" onClick={copyName} title="Copy channel name">
          {copied ? <Check size={13} /> : <CopyIcon />}
        </button>
        {slackChannelLink && (
          <button className="copy-btn" onClick={(e) => { e.stopPropagation(); BrowserOpenURL(slackChannelLink); }} title="Open channel in Slack"><ExternalLink size={12} /></button>
        )}
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
            <ChevronDown size={14} className={`expand-icon${expanded ? " expanded" : ""}`} />
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
            <ThreadRow key={t.id} thread={t} onDelete={onDeleteThread} onToggle={onToggleThread} onMarkViewed={onMarkThreadViewed} />
          ))}
        </div>
      )}

      {confirmDelete && (
        <ConfirmModal
          message={`Delete channel #${channel.channelName || channel.channelId} and all its threads?`}
          onConfirm={() => { setConfirmDelete(false); onDeleteChannel?.(channel.id); }}
          onCancel={() => setConfirmDelete(false)}
        />
      )}
    </div>
  );
}
