import { useEffect, useMemo, useRef, useState } from "react";
import { SymbolSearch, GetRepoFileTree } from "../../../wailsjs/go/main/App";

type Sym = { name: string; kind: string; path: string; language: string; line: number };

// One palette row: a file (line 1) or a symbol (its definition line).
type Row = { key: string; name: string; badge: string; path: string; line: number };

// VS Code-style quick open. The query's first character picks the mode:
//   "#query" — symbols only (⌘⇧O opens with it pre-typed)
//   "*query" — everything: files + symbols (⌘⇧F)
//   "query"  — files only (⌘O)
function parseQuery(raw: string): { mode: "files" | "symbols" | "all"; q: string } {
  if (raw.startsWith("#")) return { mode: "symbols", q: raw.slice(1) };
  if (raw.startsWith("*")) return { mode: "all", q: raw.slice(1) };
  return { mode: "files", q: raw };
}

// Rank: basename prefix < basename substring < path substring. Case-insensitive.
export function filterFiles(paths: string[], q: string, limit: number): string[] {
  const needle = q.toLowerCase();
  if (!needle) return [];
  const scored: { path: string; score: number }[] = [];
  for (const p of paths) {
    const lower = p.toLowerCase();
    const base = lower.slice(lower.lastIndexOf("/") + 1);
    let score: number;
    if (base.startsWith(needle)) score = 0;
    else if (base.includes(needle)) score = 1;
    else if (lower.includes(needle)) score = 2;
    else continue;
    scored.push({ path: p, score });
  }
  return scored
    .sort((a, b) => a.score - b.score || a.path.length - b.path.length)
    .slice(0, limit)
    .map((s) => s.path);
}

export function SymbolPalette({ repoPath, open, initialQuery = "", onClose, onPick }: {
  repoPath: string; open: boolean; initialQuery?: string; onClose: () => void;
  onPick: (path: string, line: number) => void;
}) {
  const [query, setQuery] = useState("");
  const [files, setFiles] = useState<string[]>([]);
  const [syms, setSyms] = useState<Sym[]>([]);
  const [highlight, setHighlight] = useState(0);
  const [ctagsMissing, setCtagsMissing] = useState(false);
  const debounce = useRef<ReturnType<typeof setTimeout> | null>(null);

  const { mode, q } = parseQuery(query);

  useEffect(() => {
    if (open) { setQuery(initialQuery); setSyms([]); setHighlight(0); setCtagsMissing(false); }
  }, [open, initialQuery]);

  // File list loaded once per open — filtering is client-side and instant.
  useEffect(() => {
    if (!open) return;
    GetRepoFileTree(repoPath).then((f) => setFiles((f as string[]) ?? [])).catch(() => setFiles([]));
  }, [open, repoPath]);

  // Debounced symbol search-as-you-type (150ms); files need no round-trip.
  useEffect(() => {
    if (!open || !q || mode === "files") { setSyms([]); return; }
    if (debounce.current) clearTimeout(debounce.current);
    debounce.current = setTimeout(() => {
      SymbolSearch(repoPath, q, 50)
        .then((r) => { setSyms((r as Sym[]) ?? []); setCtagsMissing(false); })
        .catch((e) => { if (String(e).includes("ctags-not-found")) setCtagsMissing(true); });
    }, 150);
    return () => { if (debounce.current) clearTimeout(debounce.current); };
  }, [q, mode, open, repoPath]);

  const rows = useMemo<Row[]>(() => {
    const fileRows = (mode === "symbols" ? [] : filterFiles(files, q, mode === "all" ? 20 : 50))
      .map((p) => ({ key: `f:${p}`, name: p.slice(p.lastIndexOf("/") + 1), badge: "file", path: p, line: 1 }));
    const symRows = (mode === "files" ? [] : syms)
      .map((s) => ({ key: `s:${s.path}:${s.line}:${s.name}`, name: s.name, badge: s.kind, path: s.path, line: s.line }));
    return [...fileRows, ...symRows];
  }, [mode, q, files, syms]);

  useEffect(() => { setHighlight(0); }, [query]);

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
  const placeholder = mode === "symbols" ? "Search symbols…" : mode === "all" ? "Search files and symbols…" : "Search files…";
  return (
    <div role="dialog" aria-modal="true" aria-label="Quick open" onClick={onClose}
      style={{ position: "fixed", inset: 0, background: "rgba(0,0,0,0.4)", display: "flex", alignItems: "flex-start", justifyContent: "center", paddingTop: "15vh", zIndex: 60 }}>
      <div onClick={(e) => e.stopPropagation()} className="modal" style={{ width: 560, maxWidth: "90%" }}>
        <input autoFocus placeholder={placeholder} value={query} onChange={(e) => setQuery(e.target.value)}
          style={{ width: "100%", boxSizing: "border-box" }} />
        <div style={{ fontSize: 10, color: "var(--text-muted)", padding: "4px 4px 0" }}>
          type <code>#</code> for symbols · <code>*</code> for everything · plain text for files
        </div>
        {ctagsMissing && (
          <div style={{ fontSize: 11, color: "var(--text-muted)", padding: "8px 4px" }}>
            Code navigation needs Universal Ctags: <code>brew install universal-ctags</code>
          </div>
        )}
        <div style={{ maxHeight: 320, overflowY: "auto", marginTop: 8 }}>
          {rows.map((s, i) => (
            <button key={s.key} role="option" aria-selected={i === highlight}
              onMouseEnter={() => setHighlight(i)} onClick={() => { onPick(s.path, s.line); onClose(); }}
              style={{ display: "flex", gap: 8, width: "100%", textAlign: "left", padding: "6px 8px", border: "none", cursor: "pointer",
                background: i === highlight ? "var(--surface2)" : "transparent", color: "var(--text)", borderRadius: 4 }}>
              <span style={{ fontWeight: 600, fontSize: 12, whiteSpace: "nowrap" }}>{s.name}</span>
              <span className="pr-status-badge" style={{ fontSize: 9 }}>{s.badge}</span>
              <span style={{ marginLeft: "auto", fontSize: 11, color: "var(--text-muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                {s.path}{s.badge === "file" ? "" : `:${s.line}`}
              </span>
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
