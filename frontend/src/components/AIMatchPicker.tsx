import { useState } from "react";
import { GitBranch } from "lucide-react";

export type AIMatchCandidate = { platform: string; ownerRepo: string; branch: string; label: string };

const keyOf = (m: AIMatchCandidate) => `${m.platform}:${m.ownerRepo}:${m.branch}`;

/**
 * Confirmation list for AI-matched branches: nothing is linked until the user
 * checks what they want and hits Accept. All candidates start checked.
 */
export function AIMatchPicker({ matches, onAccept, onCancel, accepting }: {
  matches: AIMatchCandidate[];
  onAccept: (selected: AIMatchCandidate[]) => void;
  onCancel: () => void;
  accepting?: boolean;
}) {
  const [checked, setChecked] = useState<Set<string>>(() => new Set(matches.map(keyOf)));
  const toggle = (k: string) =>
    setChecked((prev) => {
      const next = new Set(prev);
      if (next.has(k)) next.delete(k); else next.add(k);
      return next;
    });
  const selected = matches.filter((m) => checked.has(keyOf(m)));

  return (
    <div style={{ border: "1px solid var(--border)", borderRadius: "var(--radius-sm)", padding: "8px 10px", margin: "6px 0", background: "var(--surface2, rgba(127,127,127,0.06))" }}>
      <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 6 }}>
        AI Match found {matches.length} branch{matches.length === 1 ? "" : "es"} — pick which to link:
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: 4, maxHeight: 220, overflowY: "auto" }}>
        {matches.map((m) => {
          const k = keyOf(m);
          return (
            <label key={k} style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12, cursor: "pointer", userSelect: "none", minWidth: 0 }}>
              <input type="checkbox" checked={checked.has(k)} onChange={() => toggle(k)} style={{ accentColor: "var(--accent)", flexShrink: 0 }} />
              <span style={{ fontWeight: 600, flexShrink: 0 }}>{m.ownerRepo}</span>
              <span style={{ display: "inline-flex", alignItems: "center", gap: 3, fontFamily: "monospace", fontSize: 11, color: "var(--text-muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={m.branch}>
                <GitBranch size={11} style={{ flexShrink: 0 }} /> {m.branch}
              </span>
              {m.label && <span style={{ fontSize: 10, color: "var(--text-muted)", flexShrink: 0 }}>{m.label}</span>}
            </label>
          );
        })}
      </div>
      <div style={{ display: "flex", gap: 6, marginTop: 8 }}>
        <button className="btn-primary" style={{ fontSize: 11, padding: "2px 10px" }}
          disabled={selected.length === 0 || accepting}
          onClick={() => onAccept(selected)}>
          {accepting ? "Adding…" : `Accept (${selected.length})`}
        </button>
        <button className="btn-secondary" style={{ fontSize: 11, padding: "2px 10px" }} disabled={accepting} onClick={onCancel}>
          Cancel
        </button>
      </div>
    </div>
  );
}
