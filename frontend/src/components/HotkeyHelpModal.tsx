import { useEffect, useMemo, useRef, useState } from "react";
import { Pencil, RotateCcw, Play } from "lucide-react";
import { useCommandDispatch, useCommandList, useHotkey, useSuspendHotkeys } from "../hotkeys/HotkeyProvider";
import { captureCombo, type PendingCombo } from "./KeybindingRow";
import { UpdateKeybinding, ResetKeybinding } from "../../wailsjs/go/main/App";

const MOD_SYMBOLS: Record<string, string> = { cmd: "⌘", shift: "⇧", ctrl: "⌃", alt: "⌥" };
const KEY_SYMBOLS: Record<string, string> = { enter: "↵", space: "␣" };
const comboKeys = (c: PendingCombo): string[] => [
  ...(c.modifierKeys ? c.modifierKeys.split(",").map((m) => MOD_SYMBOLS[m.trim()] ?? m.toUpperCase()) : []),
  KEY_SYMBOLS[c.keyCode] ?? c.keyCode.toUpperCase(),
  ...(c.sequence ? [KEY_SYMBOLS[c.sequence] ?? c.sequence.toUpperCase()] : []),
];

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
  const runCommand = useCommandDispatch();
  const suspend = useSuspendHotkeys();

  // Inline rebinding: `capturingId` is the command whose combo we're capturing;
  // `pending` is the combo captured so far (awaiting Save, or a trailing key).
  const [capturingId, setCapturingId] = useState<string | null>(null);
  const [pending, setPending] = useState<PendingCombo | null>(null);
  const pendingRef = useRef<PendingCombo | null>(null);
  pendingRef.current = pending;

  useHotkey("help.open", () => setOpen(true));

  const stopCapturing = () => { setCapturingId(null); setPending(null); };

  const saveBinding = (id: string, combo: PendingCombo) => {
    void UpdateKeybinding(id, combo.modifierKeys, combo.keyCode, combo.sequence)
      .then(() => window.dispatchEvent(new CustomEvent("keybindings-changed")))
      .catch(() => {});
    stopCapturing();
  };

  const resetBinding = (id: string) => {
    void ResetKeybinding(id)
      .then(() => window.dispatchEvent(new CustomEvent("keybindings-changed")))
      .catch(() => {});
  };

  // While capturing, the global dispatcher is suspended and this listener owns
  // the keyboard: a modified combo sets the base chord (keep listening for an
  // optional trailing plain key that becomes a leader `sequence`); Escape cancels.
  useEffect(() => {
    if (!capturingId) return;
    suspend(true);
    const handler = (e: KeyboardEvent) => {
      e.preventDefault();
      e.stopPropagation();
      const key = e.key.toLowerCase();
      if (key === "escape") { stopCapturing(); return; }
      if (["meta", "control", "shift", "alt"].includes(key)) return;
      if (e.metaKey || e.ctrlKey || e.altKey) {
        const combo = captureCombo(e);
        if (combo) setPending({ ...combo, sequence: "" });
        return;
      }
      // Plain key after a base combo → leader sequence, then save immediately.
      if (pendingRef.current) saveBinding(capturingId, { ...pendingRef.current, sequence: key });
    };
    window.addEventListener("keydown", handler, true);
    return () => { window.removeEventListener("keydown", handler, true); suspend(false); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [capturingId]);

  useEffect(() => {
    if (open) {
      setQuery("");
      setHighlight(0);
      stopCapturing();
      // Focus the filter input once the dialog mounts.
      requestAnimationFrame(() => filterRef.current?.focus());
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
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

  // Run the given row's command and close the palette. Closing only on a
  // successful run keeps the palette open if nothing was registered to handle
  // the command (so the user isn't left wondering why the window vanished).
  const execute = (row: Row | undefined) => {
    if (!row) return;
    if (runCommand(row.id)) setOpen(false);
  };

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (capturingId) return; // the rebinding capture listener owns the keyboard
      if (e.key === "Escape") {
        e.preventDefault();
        setOpen(false);
        return;
      }
      if (e.key === "Enter") {
        e.preventDefault();
        execute(filtered[highlight]);
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, filtered, highlight, capturingId]);

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
                const isCapturing = capturingId === r.id;
                const idx = rowIndex;
                return (
                  <div
                    key={r.id}
                    className={
                      "hotkey-help-row" + (isHighlighted ? " hotkey-help-row--highlighted" : "")
                    }
                    onMouseEnter={() => { if (!capturingId) setHighlight(idx); }}
                    onClick={() => { if (!isCapturing) execute(r); }}
                    style={{ cursor: isCapturing ? "default" : "pointer" }}
                  >
                    <span className="hotkey-help-chord">
                      {isCapturing ? (
                        pending ? (
                          comboKeys(pending).map((k, i) => (
                            <kbd key={i} className="hotkey-kbd">{k}</kbd>
                          ))
                        ) : (
                          <span className="hotkey-help-listening">Press a combination…</span>
                        )
                      ) : (
                        r.chord
                      )}
                    </span>
                    <span className="hotkey-help-path">
                      {r.objectDisplay} · {r.actionDisplay}
                    </span>
                    <span className="hotkey-help-label">{r.label}</span>
                    <span className="hotkey-help-description">{r.description}</span>
                    <span className="hotkey-help-actions" onClick={(e) => e.stopPropagation()}>
                      {isCapturing ? (
                        <>
                          {pending && (
                            <button className="btn-primary hotkey-help-action" style={{ fontSize: 12, padding: "3px 10px" }} onClick={() => saveBinding(r.id, pending)}>Save</button>
                          )}
                          <button className="btn-secondary hotkey-help-action" style={{ fontSize: 12, padding: "3px 10px" }} onClick={stopCapturing}>Cancel</button>
                        </>
                      ) : (
                        isHighlighted && (
                          <>
                            <button className="btn-primary hotkey-help-action" style={{ fontSize: 12, padding: "3px 10px" }} title="Execute (↵)" onClick={() => execute(r)}>
                              <Play size={12} /> Execute
                            </button>
                            <button className="btn-icon hotkey-help-action" title="Change shortcut" onClick={() => { setHighlight(idx); setPending(null); setCapturingId(r.id); }}>
                              <Pencil size={13} />
                            </button>
                            <button className="btn-icon hotkey-help-action" title="Reset to default" onClick={() => resetBinding(r.id)}>
                              <RotateCcw size={13} />
                            </button>
                          </>
                        )
                      )}
                    </span>
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
