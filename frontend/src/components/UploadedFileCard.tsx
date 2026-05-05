import { ChevronDown } from "lucide-react";
import { useState } from "react";
import type { UploadedFile } from "../types";
import ConfirmModal from "./ConfirmModal";

function FileIcon({ fileType }: { fileType: string }) {
  const t = fileType.toLowerCase();

  if (t === "pdf") {
    return (
      <svg className="uploaded-file-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M6 2a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6H6Z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round"/>
        <path d="M14 2v6h6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
        <path d="M8 13h3M8 16h5M8 19h4" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round"/>
      </svg>
    );
  }

  if (t === "docx" || t === "doc") {
    return (
      <svg className="uploaded-file-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M6 2a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6H6Z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round"/>
        <path d="M14 2v6h6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
        <path d="M8 13h8M8 16h5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
      </svg>
    );
  }

  if (t === "rtf" || t === "odt") {
    return (
      <svg className="uploaded-file-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M6 2a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6H6Z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round"/>
        <path d="M14 2v6h6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
        <path d="M8 13h8M8 16h8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
      </svg>
    );
  }

  return (
    <svg className="uploaded-file-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M6 2a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6H6Z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round"/>
      <path d="M14 2v6h6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
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

function formatDate(iso: string): string {
  if (!iso) return "";
  try {
    return new Date(iso).toLocaleString(undefined, {
      month: "short", day: "numeric", year: "numeric",
    });
  } catch {
    return iso;
  }
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

interface Props {
  file: UploadedFile;
  onDelete?: (id: string) => void;
  onToggle?: (id: string) => void;
}

export default function UploadedFileCard({ file, onDelete, onToggle }: Props) {
  const [expanded, setExpanded] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const isInactive = !file.isActive;
  const hasContent = (file.extractedText ?? "").length > 0;

  const handleDelete = (e: React.MouseEvent) => {
    e.stopPropagation();
    setConfirmDelete(true);
  };

  return (
    <div className={`uploaded-file-card${isInactive ? " uploaded-file-card--inactive" : ""}`}>
      <div
        className="uploaded-file-header uploaded-file-header--clickable"
        onClick={() => hasContent && setExpanded(!expanded)}
      >
        {onToggle && (
          <ActiveToggle isActive={file.isActive} onToggle={(e) => { e.stopPropagation(); onToggle(file.id); }} />
        )}
        <FileIcon fileType={file.fileType} />
        <span className={`uploaded-file-name${isInactive ? " item--inactive" : ""}`}>{file.originalName}</span>
        <div className="uploaded-file-meta">
          <span className="uploaded-file-date">{formatDate(file.createdAt)}</span>
          <span className="uploaded-file-type-badge">{file.fileType.toUpperCase()}</span>
          <span className="uploaded-file-size">{formatSize(file.fileSize)}</span>
          {hasContent && <ChevronDown size={14} className={`expand-icon${expanded ? " expanded" : ""}`} />}
          {onDelete && (
            <button
              className="card-delete-btn"
              onClick={handleDelete}
              title="Remove uploaded file"
            >
              <TrashIcon />
            </button>
          )}
        </div>
      </div>

      {expanded && (
        <div className="uploaded-file-body">
          {hasContent ? (
            <pre className="uploaded-file-content">{file.extractedText}</pre>
          ) : (
            <div className="uploaded-file-empty">
              No text could be extracted from this file.
            </div>
          )}
        </div>
      )}

      {confirmDelete && (
        <ConfirmModal
          message={`Delete file "${file.originalName}"?`}
          onConfirm={() => { setConfirmDelete(false); onDelete?.(file.id); }}
          onCancel={() => setConfirmDelete(false)}
        />
      )}
    </div>
  );
}
