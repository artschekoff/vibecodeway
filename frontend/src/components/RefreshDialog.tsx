import { useState } from "react";
import Spinner from "./Spinner";
import type { SearchDiff, SearchResult } from "../types";

interface Props {
  diff: SearchDiff;
  onConfirm: (selectedKeys: string[]) => Promise<void>;
  onCancel: () => void;
}

export default function RefreshDialog({ diff, onConfirm, onCancel }: Props) {
  const newIssues = diff.new ?? [];
  const existingIssues = diff.existing ?? [];

  const [selected, setSelected] = useState<Set<string>>(
    new Set(newIssues.map((r) => r.key))
  );
  const [saving, setSaving] = useState(false);

  const toggle = (key: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const toggleAll = () => {
    if (selected.size === newIssues.length) setSelected(new Set());
    else setSelected(new Set(newIssues.map((r) => r.key)));
  };

  const handleConfirm = async () => {
    if (selected.size === 0) {
      onCancel();
      return;
    }
    setSaving(true);
    try {
      await onConfirm([...selected]);
    } finally {
      setSaving(false);
    }
  };

  const allNewSelected = selected.size === newIssues.length;

  return (
    <div className="modal-overlay" onClick={onCancel}>
      <div className="modal" style={{ width: 560 }} onClick={(e) => e.stopPropagation()}>
        <h2>Refresh Context</h2>

        {newIssues.length === 0 && existingIssues.length === 0 ? (
          <p style={{ color: "var(--text-muted)", fontSize: 13, marginBottom: 16 }}>
            No issues found for this search query.
          </p>
        ) : (
          <>
            <p style={{ color: "var(--text-muted)", fontSize: 13, marginBottom: 12 }}>
              Found <strong style={{ color: "var(--text)" }}>{newIssues.length} new</strong> and{" "}
              <strong style={{ color: "var(--text)" }}>{existingIssues.length} existing</strong> issues.
              Select which new issues to add.
            </p>

            <div className="refresh-list">
              {newIssues.length > 0 && (
                <>
                  <div className="refresh-section-label">
                    New Issues — select to add
                  </div>
                  {newIssues.length > 1 && (
                    <div className="refresh-select-all" onClick={toggleAll}>
                      <input
                        type="checkbox"
                        checked={allNewSelected}
                        onChange={toggleAll}
                        onClick={(e) => e.stopPropagation()}
                      />
                      {allNewSelected ? "Deselect all" : "Select all new"}
                    </div>
                  )}
                  {newIssues.map((r) => (
                    <IssueRow
                      key={r.key}
                      item={r}
                      checked={selected.has(r.key)}
                      onToggle={() => toggle(r.key)}
                    />
                  ))}
                </>
              )}

              {existingIssues.length > 0 && (
                <>
                  <div className="refresh-section-label">
                    Already in context
                  </div>
                  {existingIssues.map((r) => (
                    <IssueRow
                      key={r.key}
                      item={r}
                      checked={false}
                      disabled
                      onToggle={() => {}}
                    />
                  ))}
                </>
              )}
            </div>
          </>
        )}

        <div className="modal-footer">
          <button className="btn-secondary" onClick={onCancel}>Cancel</button>
          <button
            className="btn-primary"
            onClick={handleConfirm}
            disabled={saving}
          >
            {saving ? (
              <span className="btn-with-spinner">
                <Spinner size={14} color="#fff" />
                Adding…
              </span>
            ) : selected.size > 0 ? `Add ${selected.size} issue${selected.size !== 1 ? "s" : ""}` : newIssues.length === 0 ? "Close" : "Skip"}
          </button>
        </div>
      </div>
    </div>
  );
}

function IssueRow({
  item, checked, disabled, onToggle,
}: {
  item: SearchResult;
  checked: boolean;
  disabled?: boolean;
  onToggle: () => void;
}) {
  return (
    <div
      className={`refresh-item ${disabled ? "existing" : ""}`}
      onClick={disabled ? undefined : onToggle}
    >
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={onToggle}
        onClick={(e) => e.stopPropagation()}
      />
      <span className="refresh-item-key">{item.key}</span>
      <span className="refresh-item-summary">{item.summary}</span>
      <span className="refresh-item-status">{item.status}</span>
    </div>
  );
}
