import React, { useRef, useState, useCallback, useEffect } from "react";
import { SearchMessages } from "../../wailsjs/go/main/App";
import type { MentionItem } from "../types/index";

interface AttachedFile { name: string; content: string; }

interface InstructionInputProps {
  contextId: string;
  value: string;
  onChange: (value: string) => void;
  onApply: (fullPrompt: string) => Promise<void>;
  onCancel: () => void;
  isLoading?: boolean;
  disabled?: boolean;
  label?: string;
  mentionItems?: MentionItem[];
}

export function InstructionInput({
  contextId, value, onChange, onApply, onCancel,
  isLoading = false, disabled = false, label,
  mentionItems,
}: InstructionInputProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const [mentionQuery, setMentionQuery] = useState("");
  const [mentionStart, setMentionStart] = useState(-1);
  const [showDropdown, setShowDropdown] = useState(false);
  // backend results (Jira/Slack messages converted to MentionItem)
  const [backendResults, setBackendResults] = useState<MentionItem[]>([]);
  const [dropdownIdx, setDropdownIdx] = useState(0);
  const [isSearching, setIsSearching] = useState(false);
  const [attachedItems, setAttachedItems] = useState<MentionItem[]>([]);
  const [attachedFiles, setAttachedFiles] = useState<AttachedFile[]>([]);

  // Compute local filtered items synchronously (no state needed)
  const localFiltered: MentionItem[] = (mentionStart >= 0 && mentionItems)
    ? mentionItems.filter(item =>
        !mentionQuery || item.title.toLowerCase().includes(mentionQuery.toLowerCase())
      )
    : [];

  const allDropdownItems: MentionItem[] = [...localFiltered, ...backendResults];

  const searchTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const runSearch = useCallback((q: string) => {
    if (searchTimer.current) clearTimeout(searchTimer.current);
    if (!q) { setBackendResults([]); return; }
    searchTimer.current = setTimeout(async () => {
      setIsSearching(true);
      try {
        type RawMsg = { id: string; title: string; body: string; sourceId: string; sourceLabel: string; rawAuthor: string; createdAt: string };
        const r = (await SearchMessages(contextId, q)) as RawMsg[] | null;
        const mapped: MentionItem[] = (r ?? []).map(msg => ({
          id: `msg-${msg.id}`,
          title: msg.title || "(no title)",
          subtitle: `${msg.sourceLabel || msg.sourceId} · ${msg.rawAuthor}`,
          content: `[${msg.sourceLabel || msg.sourceId}: ${msg.title}]\n${msg.body}`,
          kind: "message" as const,
        }));
        setBackendResults(mapped);
        setDropdownIdx(0);
      } finally { setIsSearching(false); }
    }, 150);
  }, [contextId]);

  const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value;
    onChange(val);
    const cursor = e.target.selectionStart ?? val.length;
    // Unicode-safe regex: match @ followed by any non-whitespace non-@ chars
    const m = val.slice(0, cursor).match(/@([^\s@]*)$/);
    if (m) {
      const q = m[1];
      setMentionQuery(q);
      setMentionStart(cursor - m[0].length);
      runSearch(q);
      // show dropdown: always if we have local items, or if there's a query for backend
      const hasLocal = mentionItems && mentionItems.length > 0;
      setShowDropdown(hasLocal ? true : q.length > 0);
      setDropdownIdx(0);
    } else {
      setMentionStart(-1);
      setMentionQuery("");
      setBackendResults([]);
      setShowDropdown(false);
    }
  };

  const select = (item: MentionItem) => {
    // Remove the @token from textarea value
    if (mentionStart >= 0) {
      onChange(value.slice(0, mentionStart) + value.slice(mentionStart + 1 + mentionQuery.length));
    }
    setShowDropdown(false);
    setMentionQuery("");
    setMentionStart(-1);
    setBackendResults([]);
    setAttachedItems(prev => prev.some(x => x.id === item.id) ? prev : [...prev, item]);
    textareaRef.current?.focus();
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (!showDropdown) return;
    if (e.key === "ArrowDown") { e.preventDefault(); setDropdownIdx(i => Math.min(i + 1, allDropdownItems.length - 1)); }
    else if (e.key === "ArrowUp") { e.preventDefault(); setDropdownIdx(i => Math.max(i - 1, 0)); }
    else if ((e.key === "Enter" || e.key === "Tab") && allDropdownItems[dropdownIdx]) { e.preventDefault(); select(allDropdownItems[dropdownIdx]); }
    else if (e.key === "Escape") { setShowDropdown(false); setMentionStart(-1); }
  };

  const handleFiles = (e: React.ChangeEvent<HTMLInputElement>) => {
    Array.from(e.target.files ?? []).forEach(f => {
      if (f.size > 500_000) { console.warn(`File ${f.name} is too large (max 500 KB), skipping`); return; }
      const r = new FileReader();
      r.onload = ev => setAttachedFiles(p => p.some(x => x.name === f.name) ? p : [...p, { name: f.name, content: ev.target?.result as string }]);
      r.readAsText(f);
    });
    e.target.value = "";
  };

  const [localLoading, setLocalLoading] = useState(false);
  const isApplying = isLoading || localLoading;

  const handleApply = async () => {
    const parts = [
      ...attachedItems.map(item => item.content),
      ...attachedFiles.map(f => `[File: ${f.name}]\n${f.content}`),
    ];
    const full = parts.length ? `${parts.join("\n\n")}\n\n${value}` : value;
    setLocalLoading(true);
    try {
      await onApply(full);
      setAttachedItems([]);
      setAttachedFiles([]);
    } finally {
      setLocalLoading(false);
    }
  };

  useEffect(() => {
    const h = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setShowDropdown(false);
        setMentionStart(-1);
      }
    };
    document.addEventListener("mousedown", h);
    return () => document.removeEventListener("mousedown", h);
  }, []);

  const chip = (chipLabel: string, onRemove: () => void) => (
    <div key={chipLabel} style={{ display: "flex", alignItems: "center", gap: 4, background: "var(--surface2)", border: "1px solid var(--border)", borderRadius: 4, padding: "2px 6px", fontSize: 11, maxWidth: 220 }}>
      <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: "var(--text-muted)" }}>{chipLabel}</span>
      <button style={{ background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)", padding: 0, fontSize: 11 }} onClick={onRemove}>×</button>
    </div>
  );

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
      {label && <div style={{ fontSize: 11, color: "var(--text-muted)" }}>{label}</div>}
      {attachedItems.length > 0 && (
        <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
          {attachedItems.map(item => chip(
            `@${item.title}`,
            () => setAttachedItems(prev => prev.filter(x => x.id !== item.id))
          ))}
        </div>
      )}
      {attachedFiles.length > 0 && (
        <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
          {attachedFiles.map(f => chip(`📎 ${f.name}`, () => setAttachedFiles(p => p.filter(x => x.name !== f.name))))}
        </div>
      )}

      <div style={{ position: "relative" }}>
        {/* Stays a real textarea (not ProseEditor): the @-mention dropdown reads
            selectionStart off this ref, which Monaco does not expose. Typography
            comes from the global `textarea` rule so it matches the other boxes. */}
        <textarea
          ref={textareaRef}
          style={{ width: "100%", minHeight: 72, resize: "vertical", padding: "8px 10px", boxSizing: "border-box", background: "var(--surface2)", border: "1px solid var(--border)", borderRadius: 6, color: "var(--text)", outline: "none" }}
          placeholder="Your instruction… (type @ to reference a context item)"
          value={value} onChange={handleChange} onKeyDown={handleKeyDown}
          disabled={disabled || isApplying} autoFocus
        />
        {showDropdown && (
          <div ref={dropdownRef} style={{ position: "absolute", top: "100%", left: 0, right: 0, zIndex: 100, background: "var(--surface)", border: "1px solid var(--border)", borderRadius: 6, boxShadow: "0 4px 16px rgba(0,0,0,0.18)", maxHeight: 200, overflowY: "auto" }}>
            {isSearching && !localFiltered.length && (
              <div style={{ padding: "8px 12px", fontSize: 11, color: "var(--text-muted)" }}>Searching…</div>
            )}
            {allDropdownItems.map((item, i) => (
              <div key={item.id} onMouseDown={e => { e.preventDefault(); select(item); }}
                style={{ padding: "7px 12px", cursor: "pointer", background: i === dropdownIdx ? "var(--accent)" : "transparent", color: i === dropdownIdx ? "#fff" : "var(--text)", fontSize: 12, display: "flex", flexDirection: "column", gap: 2 }}>
                <span style={{ fontWeight: 500, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{item.title}</span>
                <span style={{ fontSize: 10, opacity: 0.7, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{item.subtitle}</span>
              </div>
            ))}
            {!isSearching && !allDropdownItems.length && (
              <div style={{ padding: "8px 12px", fontSize: 11, color: "var(--text-muted)" }}>
                {mentionQuery ? `No items matching "${mentionQuery}"` : "No items available"}
              </div>
            )}
          </div>
        )}
      </div>

      <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
        <button className="btn-primary" style={{ fontSize: 12, padding: "3px 12px", minWidth: 72 }} disabled={!value.trim() || disabled || isApplying} onClick={() => void handleApply()}>
          {isApplying
            ? <><span style={{ width: 10, height: 10, borderRadius: "50%", border: "2px solid rgba(255,255,255,0.3)", borderTopColor: "#fff", display: "inline-block", animation: "spin 0.6s linear infinite", flexShrink: 0 }} />Applying…</>
            : "Apply"}
        </button>
        <button className="btn-secondary" style={{ fontSize: 12, padding: "3px 10px" }} onClick={onCancel}>Cancel</button>
        <button className="btn-secondary" style={{ fontSize: 11, padding: "3px 8px", marginLeft: "auto" }} onClick={() => fileInputRef.current?.click()} disabled={isApplying} title="Attach files">📎</button>
        <input ref={fileInputRef} type="file" multiple style={{ display: "none" }} onChange={handleFiles} />
      </div>
    </div>
  );
}
