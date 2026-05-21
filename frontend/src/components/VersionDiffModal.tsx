import { useMemo, useEffect } from "react";
import { diffWords } from "diff";
import type { PersonAssignmentVersion } from "../types/index";

interface Props {
  currentBody: string;
  selectedVersion: PersonAssignmentVersion;
  onConfirm: () => void | Promise<void>;
  onCancel: () => void;
  isConfirming: boolean;
  confirmError?: string;
}

export function VersionDiffModal({
  currentBody,
  selectedVersion,
  onConfirm,
  onCancel,
  isConfirming,
  confirmError,
}: Props) {
  const parts = useMemo(
    () => diffWords(currentBody, selectedVersion.body),
    [currentBody, selectedVersion.body]
  );

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !isConfirming) {
        onCancel();
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onCancel, isConfirming]);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="version-diff-title"
      onClick={isConfirming ? undefined : onCancel}
      style={{
        position: "fixed",
        inset: 0,
        background: "rgba(0,0,0,0.4)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 50,
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          background: "var(--surface)",
          border: "1px solid var(--border)",
          borderRadius: 8,
          padding: 20,
          maxWidth: 640,
          width: "90%",
          maxHeight: "80vh",
          overflowY: "auto",
        }}
      >
        <div id="version-diff-title" style={{ fontSize: 12, fontWeight: 600, marginBottom: 10 }}>
          Restore version from {selectedVersion.createdAt}
        </div>
        <pre
          style={{
            whiteSpace: "pre-wrap",
            fontSize: 12,
            lineHeight: 1.6,
            marginBottom: 16,
            color: "var(--text)",
            fontFamily: "inherit",
          }}
        >
          {parts.map((part, i) => {
            if (part.removed) {
              return (
                <del
                  key={`${i}-del`}
                  style={{ background: "#fecaca", textDecoration: "line-through" }}
                >
                  {part.value}
                </del>
              );
            }
            if (part.added) {
              return (
                <ins
                  key={`${i}-add`}
                  style={{ background: "#bbf7d0", textDecoration: "none" }}
                >
                  {part.value}
                </ins>
              );
            }
            return <span key={`${i}-eq`}>{part.value}</span>;
          })}
        </pre>
        {confirmError && (
          <div style={{ fontSize: 11, color: "#dc2626", marginBottom: 8 }}>{confirmError}</div>
        )}
        <div style={{ display: "flex", gap: 8 }}>
          <button
            className="btn-primary"
            style={{ fontSize: 11, padding: "4px 12px" }}
            disabled={isConfirming}
            onClick={() => void onConfirm()}
          >
            {isConfirming ? "Restoring…" : "Confirm"}
          </button>
          <button
            aria-label="Cancel version restore"
            className="btn-secondary"
            style={{ fontSize: 11, padding: "4px 10px" }}
            onClick={onCancel}
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}
