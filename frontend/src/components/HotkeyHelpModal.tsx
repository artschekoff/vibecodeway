import { useEffect, useMemo, useRef, useState } from "react";
import { useCommandList, useHotkey } from "../hotkeys/HotkeyProvider";

/**
 * Splits camelCase into words and capitalizes each, special-casing the
 * literal "pr" -> "PR". Used for both `object` and `action` display since
 * registry.ts's `commandLabel`/`cap()` does NOT split camelCase (e.g. it
 * would render "copyName" unsplit).
 */
function humanize(s: string): string {
  if (s.toLowerCase() === "pr") return "PR";
  const words = s
    .replace(/([A-Z])/g, " $1")
    .trim()
    .split(/\s+/)
    .filter(Boolean);
  return words
    .map((w) => (w.toLowerCase() === "pr" ? "PR" : w.charAt(0).toUpperCase() + w.slice(1)))
    .join(" ");
}

interface Row {
  id: string;
  chord: string;
  object: string;
  action: string;
  label: string;
  description: string;
  objectDisplay: string;
  actionDisplay: string;
}

export function HotkeyHelpModal() {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [highlight, setHighlight] = useState(0);
  const filterRef = useRef<HTMLInputElement | null>(null);
  const commandList = useCommandList();

  useHotkey("help.open", () => setOpen(true));

  useEffect(() => {
    if (open) {
      setQuery("");
      setHighlight(0);
      // Focus the filter input once the dialog mounts.
      requestAnimationFrame(() => filterRef.current?.focus());
    }
  }, [open]);

  const rows: Row[] = useMemo(
    () =>
      commandList.map(({ command, chord }) => ({
        id: command.id,
        chord,
        object: command.object,
        action: command.action,
        label: command.label,
        description: command.description,
        objectDisplay: humanize(command.object),
        actionDisplay: humanize(command.action),
      })),
    [commandList]
  );

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return rows;
    return rows.filter((r) =>
      [r.chord, r.objectDisplay, r.actionDisplay, r.label, r.description]
        .join(" ")
        .toLowerCase()
        .includes(q)
    );
  }, [rows, query]);

  const grouped = useMemo(() => {
    const map = new Map<string, Row[]>();
    for (const r of filtered) {
      const key = r.objectDisplay;
      const list = map.get(key);
      if (list) list.push(r);
      else map.set(key, [r]);
    }
    return [...map.entries()];
  }, [filtered]);

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
        setHighlight((h) => Math.min(h + 1, filtered.length - 1));
        return;
      }
      if (e.key === "ArrowUp") {
        e.preventDefault();
        setHighlight((h) => Math.max(h - 1, 0));
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [open, filtered.length]);

  if (!open) return null;

  let rowIndex = -1;

  return (
    <div className="modal-overlay" onClick={() => setOpen(false)}>
      <div
        className="modal modal--wide hotkey-help-modal"
        role="dialog"
        aria-modal="true"
        aria-label="Keyboard shortcuts"
        onClick={(e) => e.stopPropagation()}
      >
        <h2>Keyboard Shortcuts</h2>
        <input
          ref={filterRef}
          type="text"
          className="hotkey-help-filter"
          placeholder="Filter shortcuts…"
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setHighlight(0);
          }}
        />
        <div className="modal-scroll-body hotkey-help-body">
          {grouped.map(([groupName, groupRows]) => (
            <div key={groupName} className="hotkey-help-group">
              <div className="hotkey-help-group-title">{groupName}</div>
              {groupRows.map((r) => {
                rowIndex += 1;
                const isHighlighted = rowIndex === highlight;
                return (
                  <div
                    key={r.id}
                    className={
                      "hotkey-help-row" + (isHighlighted ? " hotkey-help-row--highlighted" : "")
                    }
                  >
                    <span className="hotkey-help-chord">{r.chord}</span>
                    <span className="hotkey-help-path">
                      {r.objectDisplay} · {r.actionDisplay}
                    </span>
                    <span className="hotkey-help-label">{r.label}</span>
                    <span className="hotkey-help-description">{r.description}</span>
                  </div>
                );
              })}
            </div>
          ))}
          {filtered.length === 0 && <div className="hotkey-help-empty">No matching shortcuts</div>}
        </div>
      </div>
    </div>
  );
}

export default HotkeyHelpModal;
