import { ChevronDown, ExternalLink } from "lucide-react";
import { useState } from "react";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";

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
  thread: SlackThread;
}

export default function SlackThreadCard({ thread }: Props) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="issue-card">
      <div className="issue-card-header" onClick={() => setExpanded(!expanded)}>
        <span className="slack-thread-hash">#</span>
        <span className="issue-summary" style={{ flex: 1 }}>
          {thread.channelName || thread.channelId}
        </span>
        <div className="issue-meta">
          <span className="channel-result-workspace">{thread.workspace}</span>
          {thread.permalink && (
            <button
              className="copy-btn"
              onClick={(e) => { e.stopPropagation(); BrowserOpenURL(thread.permalink); }}
              title="Open thread in Slack"
            >
              <ExternalLink size={12} />
            </button>
          )}
          <ChevronDown size={14} className={`expand-icon${expanded ? " expanded" : ""}`} />
        </div>
      </div>

      {expanded && (
        <div className="issue-body">
          {thread.content ? (
            <pre className="slack-thread-content">{thread.content}</pre>
          ) : (
            <div className="no-comments">No content loaded yet — click Refresh to fetch.</div>
          )}
        </div>
      )}
    </div>
  );
}
