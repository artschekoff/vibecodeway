import { useState } from "react";
import type { Issue, Comment } from "../types";

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
}

export default function IssueCard({ issue, onDeleteIssue, onDeleteComment, onToggleIssue, onToggleComment }: Props) {
  const [expanded, setExpanded] = useState(false);
  const [copied, setCopied] = useState(false);

  const copyKey = (e: React.MouseEvent) => {
    e.stopPropagation();
    navigator.clipboard.writeText(issue.jiraKey).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  };

  const handleDeleteIssue = (e: React.MouseEvent) => {
    e.stopPropagation();
    onDeleteIssue?.(issue.id);
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

  return (
    <div className={`issue-card${isInactive ? " issue-card--inactive" : ""}`}>
      <div className="issue-card-header" onClick={() => setExpanded(!expanded)}>
        {onToggleIssue && (
          <ActiveToggle isActive={issue.isActive} onToggle={(e) => { e.stopPropagation(); onToggleIssue(issue.id); }} />
        )}
        <span className={`issue-key${isInactive ? " item--inactive" : ""}`}>{issue.jiraKey}</span>
        <button className="copy-btn" onClick={copyKey} title="Copy ticket number">
          {copied ? "✓" : <CopyIcon />}
        </button>
        <span className={`issue-summary${isInactive ? " item--inactive" : ""}`}>{issue.summary}</span>
        <div className="issue-meta">
          {issue.assignee && <span className="issue-assignee">@{issue.assignee}</span>}
          <span className="status-badge">{issue.status}</span>
          <span className="status-badge" style={{ borderColor: "transparent", background: "transparent" }}>
            {comments.length} comment{comments.length !== 1 ? "s" : ""}
          </span>
          <span className={`expand-icon ${expanded ? "expanded" : ""}`}>▼</span>
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
                <div key={c.id} className={`comment${!c.isActive ? " comment--inactive" : ""}`}>
                  <div className="comment-header">
                    {onToggleComment && (
                      <ActiveToggle isActive={c.isActive} onToggle={() => onToggleComment(c.id)} />
                    )}
                    <span className={`comment-author${!c.isActive ? " item--inactive" : ""}`}>{c.author || "Unknown"}</span>
                    <span className="comment-date">{formatDate(c.jiraCreated)}</span>
                    {onDeleteComment && (
                      <button
                        className="card-delete-btn card-delete-btn--sm"
                        onClick={() => onDeleteComment(c.id)}
                        title="Remove this comment"
                      >
                        <TrashIcon />
                      </button>
                    )}
                  </div>
                  <div className={`comment-body${!c.isActive ? " item--inactive" : ""}`}>{c.body}</div>
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}
