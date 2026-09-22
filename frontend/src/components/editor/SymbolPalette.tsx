import { useEffect, useRef, useState } from "react";
import { SymbolSearch } from "../../../wailsjs/go/main/App";

type Sym = { name: string; kind: string; path: string; language: string; line: number };

export function SymbolPalette({ repoPath, open, onClose, onPick }: {
  repoPath: string; open: boolean; onClose: () => void;
  onPick: (path: string, line: number) => void;
}) {
  const [query, setQuery] = useState("");
  const [rows, setRows] = useState<Sym[]>([]);
  const [highlight, setHighlight] = useState(0);
  const [ctagsMissing, setCtagsMissing] = useState(false);
  const debounce = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => { if (open) { setQuery(""); setRows([]); setHighlight(0); } }, [open]);

  // Debounced search-as-you-type (150ms).
  useEffect(() => {
    if (!open || !query) { setRows([]); return; }
    if (debounce.current) clearTimeout(debounce.current);
    debounce.current = setTimeout(() => {
      SymbolSearch(repoPath, query, 50)
        .then((r) => { setRows((r as Sym[]) ?? []); setHighlight(0); setCtagsMissing(false); })
        .catch((e) => { if (String(e).includes("ctags-not-found")) setCtagsMissing(true); });
    }, 150);
  }, [query, open, repoPath]);

  // ↑/↓/Enter/Esc — capture phase so keys never leak to the page below.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "ArrowDown") { e.preventDefault(); e.stopPropagation(); setHighlight((i) => Math.min(i + 1, rows.length - 1)); }
      else if (e.key === "ArrowUp") { e.preventDefault(); e.stopPropagation(); setHighlight((i) => Math.max(i - 1, 0)); }
      else if (e.key === "Enter") { e.preventDefault(); e.stopPropagation(); const s = rows[highlight]; if (s) { onPick(s.path, s.line); onClose(); } }
      else if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); onClose(); }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [open, rows, highlight, onPick, onClose]);

  if (!open) return null;
  return (
    <div role="dialog" aria-modal="true" aria-label="Search symbols" onClick={onClose}
      style={{ position: "fixed", inset: 0, background: "rgba(0,0,0,0.4)", display: "flex", alignItems: "flex-start", justifyContent: "center", paddingTop: "15vh", zIndex: 60 }}>
      <div onClick={(e) => e.stopPropagation()} className="modal" style={{ width: 560, maxWidth: "90%" }}>
        <input autoFocus placeholder="Search symbols…" value={query} onChange={(e) => setQuery(e.target.value)}
          style={{ width: "100%", boxSizing: "border-box" }} />
        {ctagsMissing && (
          <div style={{ fontSize: 11, color: "var(--text-muted)", padding: "8px 4px" }}>
            Code navigation needs Universal Ctags: <code>brew install universal-ctags</code>
          </div>
        )}
        <div style={{ maxHeight: 320, overflowY: "auto", marginTop: 8 }}>
          {rows.map((s, i) => (
            <button key={`${s.path}:${s.line}:${s.name}`} role="option" aria-selected={i === highlight}
              onMouseEnter={() => setHighlight(i)} onClick={() => { onPick(s.path, s.line); onClose(); }}
              style={{ display: "flex", gap: 8, width: "100%", textAlign: "left", padding: "6px 8px", border: "none", cursor: "pointer",
                background: i === highlight ? "var(--surface2)" : "transparent", color: "var(--text)", borderRadius: 4 }}>
              <span style={{ fontWeight: 600, fontSize: 12 }}>{s.name}</span>
              <span className="pr-status-badge" style={{ fontSize: 9 }}>{s.kind}</span>
              <span style={{ marginLeft: "auto", fontSize: 11, color: "var(--text-muted)" }}>{s.path}:{s.line}</span>
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
