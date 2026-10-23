import { useEffect, useRef, useState } from "react";
import { useHotkey } from "../hotkeys/HotkeyProvider";
import { useTabContext } from "../contexts/TabContext";
import { SearchContexts } from "../../wailsjs/go/main/App";
import type { store } from "../../wailsjs/go/models";

// Stable per-org tag color (self-contained; mirrors TabBar's ORG_COLORS palette).
const ORG_COLORS = ["#F5A040", "#7CD4E7", "#82D9A0", "#B49CF5", "#E07878", "#F5C97C", "#F57CA0", "#C9E882"];
function colorForOrg(orgId: string): string {
  let h = 0;
  for (let i = 0; i < orgId.length; i++) h = (h * 31 + orgId.charCodeAt(i)) | 0;
  return ORG_COLORS[Math.abs(h) % ORG_COLORS.length];
}

export function GlobalSearchModal() {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [hits, setHits] = useState<store.ContextSearchHit[]>([]);
  const [highlight, setHighlight] = useState(0);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const { openContextTab } = useTabContext();

  useHotkey("search.global", () => setOpen(true));

  useEffect(() => {
    if (!open) return;
    setQuery("");
    setHits([]);
    setHighlight(0);
    requestAnimationFrame(() => inputRef.current?.focus());
  }, [open]);

  // Debounced cross-org search.
  useEffect(() => {
    if (!open) return;
    const q = query.trim();
    if (!q) {
      setHits([]);
      return;
    }
    const t = setTimeout(() => {
      SearchContexts(q)
        .then((res) => {
          setHits(res ?? []);
          setHighlight(0);
        })
        .catch((e) => console.error("global search failed", e));
    }, 200);
    return () => clearTimeout(t);
  }, [query, open]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        setOpen(false);
        return;
      }
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setHighlight((h) => (hits.length ? (h + 1) % hits.length : 0));
        return;
      }
      if (e.key === "ArrowUp") {
        e.preventDefault();
        setHighlight((h) => (hits.length ? (h - 1 + hits.length) % hits.length : 0));
        return;
      }
      if (e.key === "Enter") {
        e.preventDefault();
        const hit = hits[highlight];
        if (hit) {
          openContextTab(hit.id, hit.key, hit.orgName, hit.orgId);
          setOpen(false);
        }
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [open, hits, highlight, openContextTab]);

  if (!open) return null;

  return (
    <div className="modal-overlay" onClick={() => setOpen(false)}>
      <div
        className="modal global-search-modal"
        role="dialog"
        aria-modal="true"
        aria-label="Global search"
        onClick={(e) => e.stopPropagation()}
      >
        <input
          ref={inputRef}
          type="text"
          className="global-search-input"
          placeholder="Search contexts across all organizations…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <div className="modal-scroll-body global-search-results">
          {hits.map((hit, i) => (
            <div
              key={hit.id}
              className={"global-search-row" + (i === highlight ? " global-search-row--highlighted" : "")}
              onMouseEnter={() => setHighlight(i)}
              onClick={() => {
                openContextTab(hit.id, hit.key, hit.orgName, hit.orgId);
                setOpen(false);
              }}
            >
              {hit.orgName && (
                <span className="global-search-org" style={{ background: colorForOrg(hit.orgId) }}>
                  {hit.orgName}
                </span>
              )}
              <span className="global-search-key">{hit.key}</span>
              {hit.summary && <span className="global-search-summary">{hit.summary}</span>}
            </div>
          ))}
          {query.trim() !== "" && hits.length === 0 && (
            <div className="global-search-empty">No matching contexts</div>
          )}
        </div>
      </div>
    </div>
  );
}
