import React, { useRef, useState, useCallback, useEffect } from "react";
import { SearchMessages } from "../../wailsjs/go/main/App";

interface ContextMessage {
  id: string; title: string; body: string;
  sourceId: string; sourceLabel: string; rawAuthor: string; createdAt: string;
}
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
}

export function InstructionInput({ contextId, value, onChange, onApply, onCancel, isLoading = false, disabled = false, label }: InstructionInputProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const [mentionQuery, setMentionQuery] = useState("");
  const [mentionStart, setMentionStart] = useState(-1);
  const [showDropdown, setShowDropdown] = useState(false);
  const [dropdownResults, setDropdownResults] = useState<ContextMessage[]>([]);
  const [dropdownIdx, setDropdownIdx] = useState(0);
  const [isSearching, setIsSearching] = useState(false);
  const [attachedMessages, setAttachedMessages] = useState<ContextMessage[]>([]);
  const [attachedFiles, setAttachedFiles] = useState<AttachedFile[]>([]);

  const searchTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const runSearch = useCallback((q: string) => {
    if (searchTimer.current) clearTimeout(searchTimer.current);
    if (!q) { setDropdownResults([]); setShowDropdown(false); return; }
    searchTimer.current = setTimeout(async () => {
      setIsSearching(true);
      try {
        const r = (await SearchMessages(contextId, q)) as ContextMessage[] | null;
        setDropdownResults(r ?? []);
        setShowDropdown((r?.length ?? 0) > 0);
        setDropdownIdx(0);
      } finally { setIsSearching(false); }
    }, 150);
  }, [contextId]);

  const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value;
    onChange(val);
    const cursor = e.target.selectionStart ?? val.length;
    const m = val.slice(0, cursor).match(/@(\w*)$/);
    if (m) { setMentionQuery(m[1]); setMentionStart(cursor - m[0].length); runSearch(m[1]); }
    else { setShowDropdown(false); setMentionQuery(""); setMentionStart(-1); }
  };

  const select = (msg: ContextMessage) => {
    if (mentionStart >= 0) onChange(value.slice(0, mentionStart) + value.slice(mentionStart + 1 + mentionQuery.length));
    setShowDropdown(false); setMentionQuery(""); setMentionStart(-1);
    setAttachedMessages(p => p.some(m => m.id === msg.id) ? p : [...p, msg]);
    textareaRef.current?.focus();
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (!showDropdown) return;
    if (e.key === "ArrowDown") { e.preventDefault(); setDropdownIdx(i => Math.min(i + 1, dropdownResults.length - 1)); }
    else if (e.key === "ArrowUp") { e.preventDefault(); setDropdownIdx(i => Math.max(i - 1, 0)); }
    else if ((e.key === "Enter" || e.key === "Tab") && dropdownResults[dropdownIdx]) { e.preventDefault(); select(dropdownResults[dropdownIdx]); }
    else if (e.key === "Escape") setShowDropdown(false);
  };

  const handleFiles = (e: React.ChangeEvent<HTMLInputElement>) => {
    Array.from(e.target.files ?? []).forEach(f => {
      if (f.size > 500_000) {
        console.warn(`File ${f.name} is too large (max 500 KB), skipping`);
        return;
      }
      const r = new FileReader();
      r.onload = ev => setAttachedFiles(p => p.some(x => x.name === f.name) ? p : [...p, { name: f.name, content: ev.target?.result as string }]);
      r.readAsText(f);
    });
    e.target.value = "";
  };

  const handleApply = async () => {
    const parts = [
      ...attachedMessages.map(m => `[${m.sourceLabel || m.sourceId}: ${m.title}]\n${m.body}`),
      ...attachedFiles.map(f => `[File: ${f.name}]\n${f.content}`),
    ];
    const full = parts.length ? `${parts.join("\n\n")}\n\n${value}` : value;
    await onApply(full);
    setAttachedMessages([]); setAttachedFiles([]);
  };

  useEffect(() => {
    const h = (e: MouseEvent) => { if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) setShowDropdown(false); };
    document.addEventListener("mousedown", h);
    return () => document.removeEventListener("mousedown", h);
  }, []);

  const chip = (label: string, onRemove: () => void) => (
    <div key={label} style={{ display: "flex", alignItems: "center", gap: 4, background: "var(--surface2)", border: "1px solid var(--border)", borderRadius: 4, padding: "2px 6px", fontSize: 11, maxWidth: 220 }}>
      <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: "var(--text-muted)" }}>{label}</span>
      <button style={{ background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)", padding: 0, fontSize: 11 }} onClick={onRemove}>×</button>
    </div>
  );

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
      {label && <div style={{ fontSize: 11, color: "var(--text-muted)" }}>{label}</div>}
      {attachedMessages.length > 0 && <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>{attachedMessages.map(m => chip(`@${m.title || m.sourceLabel}`, () => setAttachedMessages(p => p.filter(x => x.id !== m.id))))}</div>}
      {attachedFiles.length > 0 && <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>{attachedFiles.map(f => chip(`📎 ${f.name}`, () => setAttachedFiles(p => p.filter(x => x.name !== f.name))))}</div>}

      <div style={{ position: "relative" }}>
        <textarea
          ref={textareaRef}
          style={{ width: "100%", minHeight: 72, resize: "vertical", fontFamily: "inherit", fontSize: 12, textAlign: "left", padding: "8px 10px", lineHeight: 1.5, boxSizing: "border-box", background: "var(--surface2)", border: "1px solid var(--border)", borderRadius: 6, color: "var(--text)", outline: "none" }}
          placeholder="Your instruction… (type @ to reference a context message)"
          value={value} onChange={handleChange} onKeyDown={handleKeyDown}
          disabled={disabled || isLoading} autoFocus
        />
        {showDropdown && (
          <div ref={dropdownRef} style={{ position: "absolute", top: "100%", left: 0, right: 0, zIndex: 100, background: "var(--surface)", border: "1px solid var(--border)", borderRadius: 6, boxShadow: "0 4px 16px rgba(0,0,0,0.18)", maxHeight: 200, overflowY: "auto" }}>
            {isSearching && <div style={{ padding: "8px 12px", fontSize: 11, color: "var(--text-muted)" }}>Searching…</div>}
            {!isSearching && dropdownResults.map((msg, i) => (
              <div key={msg.id} onMouseDown={e => { e.preventDefault(); select(msg); }}
                style={{ padding: "7px 12px", cursor: "pointer", background: i === dropdownIdx ? "var(--accent)" : "transparent", color: i === dropdownIdx ? "#fff" : "var(--text)", fontSize: 12, display: "flex", flexDirection: "column", gap: 2 }}>
                <span style={{ fontWeight: 500, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{msg.title || "(no title)"}</span>
                <span style={{ fontSize: 10, opacity: 0.7, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{msg.sourceLabel || msg.sourceId} · {msg.rawAuthor}</span>
              </div>
            ))}
            {!isSearching && !dropdownResults.length && mentionQuery && <div style={{ padding: "8px 12px", fontSize: 11, color: "var(--text-muted)" }}>No messages matching "{mentionQuery}"</div>}
          </div>
        )}
      </div>

      <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
        <button className="btn-primary" style={{ fontSize: 12, padding: "3px 12px" }} disabled={!value.trim() || disabled || isLoading} onClick={() => void handleApply()}>
          {isLoading
            ? <span style={{ display: "flex", alignItems: "center", gap: 5 }}><span style={{ width: 10, height: 10, borderRadius: "50%", border: "2px solid rgba(255,255,255,0.3)", borderTopColor: "#fff", display: "inline-block", animation: "spin 0.6s linear infinite" }} />Applying…</span>
            : "Apply"}
        </button>
        <button className="btn-secondary" style={{ fontSize: 12, padding: "3px 10px" }} onClick={onCancel} disabled={isLoading}>Cancel</button>
        <button className="btn-secondary" style={{ fontSize: 11, padding: "3px 8px", marginLeft: "auto" }} onClick={() => fileInputRef.current?.click()} disabled={isLoading} title="Attach files">📎</button>
        <input ref={fileInputRef} type="file" multiple style={{ display: "none" }} onChange={handleFiles} />
      </div>
    </div>
  );
}
