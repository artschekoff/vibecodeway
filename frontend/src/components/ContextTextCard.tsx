import { useState } from "react";
import { ChevronDown } from "lucide-react";
import type { ContextText } from "../types";

function TextIcon() {
  return (
    <svg className="uploaded-file-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M6 2a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6H6Z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round"/>
      <path d="M14 2v6h6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
      <path d="M8 12h8M8 15h8M8 18h5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round"/>
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

function formatSize(chars: number): string {
  if (chars < 1000) return `${chars} ch`;
  return `${(chars / 1000).toFixed(1)}k ch`;
}

interface Props {
  text: ContextText;
  onDelete?: (id: string) => void;
  onToggle?: (id: string) => void;
}

export default function ContextTextCard({ text, onDelete, onToggle }: Props) {
  const [expanded, setExpanded] = useState(false);
  const isInactive = !text.isActive;

  const title = text.name.length > 100 ? text.name.slice(0, 100) + "…" : text.name;

  return (
    <div className={`uploaded-file-card${isInactive ? " uploaded-file-card--inactive" : ""}`}>
      <div
        className="uploaded-file-header uploaded-file-header--clickable"
        onClick={() => setExpanded((v) => !v)}
      >
        {onToggle && (
          <label
            className="item-active-toggle"
            onClick={(e) => e.stopPropagation()}
            title={isInactive ? "Inactive — click to activate" : "Active — click to deactivate"}
          >
            <input type="checkbox" checked={text.isActive} onChange={() => {}} onClick={(e) => { e.stopPropagation(); onToggle(text.id); }} />
            <span className={`item-active-indicator${text.isActive ? " item-active-indicator--on" : ""}`} />
          </label>
        )}
        <TextIcon />
        <span className={`uploaded-file-name${isInactive ? " item--inactive" : ""}`} title={text.name}>
          {title}
        </span>
        <div className="uploaded-file-meta">
          <span className="uploaded-file-type-badge">TEXT</span>
          <span className="uploaded-file-size">{formatSize(text.content.length)}</span>
          <ChevronDown size={14} className={`expand-icon${expanded ? " expanded" : ""}`} />
          {onDelete && (
            <button
              className="card-delete-btn"
              onClick={(e) => { e.stopPropagation(); onDelete(text.id); }}
              title="Remove text"
            >
              <TrashIcon />
            </button>
          )}
        </div>
      </div>

      {expanded && (
        <div className="uploaded-file-body">
          <pre className="uploaded-file-content">{text.content}</pre>
        </div>
      )}
    </div>
  );
}
