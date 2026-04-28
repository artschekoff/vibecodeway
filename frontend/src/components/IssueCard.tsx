import { Check, ChevronDown, ExternalLink } from "lucide-react";
import { useState } from "react";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import type { Issue, Comment } from "../types";
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

interface Props {
  issue: Issue;
  onDeleteIssue?: (id: string) => void;
  onDeleteComment?: (id: string) => void;
  onToggleIssue?: (id: string) => void;
  onToggleComment?: (id: string) => void;
  onMarkIssueViewed?: (id: string) => void;
  onMarkCommentViewed?: (id: string) => void;
  jiraBaseUrl?: string;
}

export default function IssueCard({ issue, onDeleteIssue, onDeleteComment, onToggleIssue, onToggleComment, onMarkIssueViewed, onMarkCommentViewed, jiraBaseUrl }: Props) {
  const [expanded, setExpanded] = useState(false);
  const [copied, setCopied] = useState(false);
  const [confirmIssue, setConfirmIssue] = useState(false);
  const [confirmComment, setConfirmComment] = useState<string | null>(null);

  const copyKey = (e: React.MouseEvent) => {
    e.stopPropagation();
    navigator.clipboard.writeText(issue.jiraKey).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  };

  const handleDeleteIssue = (e: React.MouseEvent) => {
    e.stopPropagation();
    setConfirmIssue(true);
  };

  const formatDate = (iso: string) => {
    if (!iso) return "";
    try {
      return new Date(iso).toLocaleString(undefined, {
        month: "short", day: "numeric", year: "numeric",
        hour: "2-digit", minute: "2-digit",
      });
    } catch {
      return iso;
    }
  };

  const comments = issue.comments ?? [];
  const isInactive = !issue.isActive;
  const isUnread = !issue.viewedAt;
  const hasUnreadComments = comments.some((c) => !c.viewedAt);
  const jiraLink = jiraBaseUrl && issue.jiraKey ? `${jiraBaseUrl}/browse/${issue.jiraKey}` : null;

  return (
    <div className={`issue-card${isInactive ? " issue-card--inactive" : ""}${isUnread || hasUnreadComments ? " item--unread-parent" : ""}`}>
      <div className={`issue-card-header${isUnread ? " item--unread" : ""}`} onClick={() => setExpanded(!expanded)}>
        {onToggleIssue && (
          <ActiveToggle isActive={issue.isActive} onToggle={(e) => { e.stopPropagation(); onToggleIssue(issue.id); }} />
        )}
        {jiraLink ? (
          <button
            className={`issue-key issue-key--link${isInactive ? " item--inactive" : ""}`}
            onClick={(e) => { e.stopPropagation(); BrowserOpenURL(jiraLink); }}
            title="Open in Jira"
          >
            {issue.jiraKey} <ExternalLink size={10} />
          </button>
        ) : (
          <span className={`issue-key${isInactive ? " item--inactive" : ""}`}>{issue.jiraKey}</span>
        )}
        <button className="copy-btn" onClick={copyKey} title="Copy ticket number">
          {copied ? <Check size={13} /> : <CopyIcon />}
        </button>
        <span className={`issue-summary${isInactive ? " item--inactive" : ""}`}>{issue.summary}</span>
        <div className="issue-meta">
          {issue.assignee && <span className="issue-assignee">@{issue.assignee}</span>}
          <span className="status-badge">{issue.status}</span>
          <span className="status-badge" style={{ borderColor: "transparent", background: "transparent" }}>
            {comments.length} comment{comments.length !== 1 ? "s" : ""}
          </span>
          <ChevronDown size={14} className={`expand-icon${expanded ? " expanded" : ""}`} />
        </div>
        <div className="card-actions">
          {isUnread && onMarkIssueViewed && (
            <button className="mark-viewed-btn" onClick={(e) => { e.stopPropagation(); onMarkIssueViewed(issue.id); }} title="Mark as viewed"><Check size={13} /></button>
          )}
          {onDeleteIssue && (
            <button
              className="card-delete-btn"
              onClick={handleDeleteIssue}
              title="Remove this issue"
            >
              <TrashIcon />
            </button>
          )}
        </div>
      </div>

      {expanded && (
        <div className="issue-body">
          {issue.description && (
            <div className="issue-description">{issue.description}</div>
          )}

          <div className="comments-section">
            <h4>Comments ({comments.length})</h4>
            {comments.length === 0 ? (
              <div className="no-comments">No comments yet</div>
            ) : (
              comments.map((c: Comment) => (
                <div key={c.id} className={`comment${!c.isActive ? " comment--inactive" : ""}${!c.viewedAt ? " item--unread" : ""}`}>
                  <div className="comment-header">
                    <div className="comment-header-left">
                      {onToggleComment && (
                        <ActiveToggle isActive={c.isActive} onToggle={() => onToggleComment(c.id)} />
                      )}
                      <span className={`comment-author${!c.isActive ? " item--inactive" : ""}`}>{c.author || "Unknown"}</span>
                      <span className="comment-date">{formatDate(c.jiraCreated)}</span>
                    </div>
                    <div className="card-actions">
                      {!c.viewedAt && onMarkCommentViewed && (
                        <button className="mark-viewed-btn" onClick={() => onMarkCommentViewed(c.id)} title="Mark as viewed"><Check size={13} /></button>
                      )}
                      {onDeleteComment && (
                        <button
                          className="card-delete-btn card-delete-btn--sm"
                          onClick={() => setConfirmComment(c.id)}
                          title="Remove this comment"
                        >
                          <TrashIcon />
                        </button>
                      )}
                    </div>
                  </div>
                  <div className={`comment-body${!c.isActive ? " item--inactive" : ""}`}>{c.body}</div>
                </div>
              ))
            )}
          </div>
        </div>
      )}

      {confirmIssue && (
        <ConfirmModal
          message={`Delete ticket ${issue.jiraKey}?`}
          onConfirm={() => { setConfirmIssue(false); onDeleteIssue?.(issue.id); }}
          onCancel={() => setConfirmIssue(false)}
        />
      )}
      {confirmComment && (
        <ConfirmModal
          message="Delete this comment?"
          onConfirm={() => { const id = confirmComment; setConfirmComment(null); onDeleteComment?.(id); }}
          onCancel={() => setConfirmComment(null)}
        />
      )}
    </div>
  );
}
