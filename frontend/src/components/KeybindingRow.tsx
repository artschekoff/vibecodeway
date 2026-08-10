import { useEffect, useRef, useState } from "react";
import { Pencil, RotateCcw } from "lucide-react";
import type { Keybinding } from "../types";

interface Props {
  binding: Keybinding;
  onSave: (command: string, modifierKeys: string, keyCode: string, sequence: string) => void;
  onReset?: (command: string) => void;
}

const MOD_SYMBOLS: Record<string, string> = {
  cmd: "⌘",
  shift: "⇧",
  ctrl: "⌃",
  alt: "⌥",
};

const KEY_SYMBOLS: Record<string, string> = {
  enter: "↵",
  space: "␣",
};

function formatModifiers(modifierKeys: string): string[] {
  if (!modifierKeys) return [];
  return modifierKeys.split(",").map((m) => MOD_SYMBOLS[m.trim()] ?? m.toUpperCase());
}

function formatKey(keyCode: string): string {
  return KEY_SYMBOLS[keyCode] ?? keyCode.toUpperCase();
}

interface PendingCombo {
  modifierKeys: string; // comma-separated
  keyCode: string;
  sequence: string; // optional trailing single key (leader chord), "" when none
}

function captureCombo(e: KeyboardEvent): Omit<PendingCombo, "sequence"> | null {
  const mods: string[] = [];
  if (e.metaKey) mods.push("cmd");
  if (e.ctrlKey) mods.push("ctrl");
  if (e.shiftKey) mods.push("shift");
  if (e.altKey) mods.push("alt");
  if (mods.length === 0) return null;

  const key = e.key.toLowerCase();
  if (["meta", "control", "shift", "alt"].includes(key)) return null;

  return { modifierKeys: mods.join(","), keyCode: key };
}

export default function KeybindingRow({ binding, onSave, onReset }: Props) {
  const [listening, setListening] = useState(false);
  const [pending, setPending] = useState<PendingCombo | null>(null);
  // Handler closure is bound when `listening` flips; mirror pending in a ref so
  // the trailing-key branch can see the base combo without re-binding.
  const pendingRef = useRef<PendingCombo | null>(pending);
  pendingRef.current = pending;

  useEffect(() => {
    if (!listening) return;

    const handler = (e: KeyboardEvent) => {
      e.preventDefault();
      if (e.key === "Escape") {
        setListening(false);
        setPending(null);
        return;
      }
      const key = e.key.toLowerCase();
      if (["meta", "control", "shift", "alt"].includes(key)) return;

      if (e.metaKey || e.ctrlKey || e.altKey) {
        // A modified combo (re)sets the base chord; keep listening for an
        // optional trailing plain key that becomes the leader `sequence`.
        const combo = captureCombo(e);
        if (combo) setPending({ ...combo, sequence: "" });
        return;
      }

      // Plain key after a base combo → leader sequence, then finalize.
      if (pendingRef.current) {
        setPending({ ...pendingRef.current, sequence: key });
        setListening(false);
      }
    };

    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [listening]);

  function handleChange() {
    setPending(null);
    setListening(true);
  }

  function handleSave() {
    if (!pending) return;
    onSave(binding.command, pending.modifierKeys, pending.keyCode, pending.sequence);
    setPending(null);
    setListening(false);
  }

  function handleCancel() {
    setPending(null);
    setListening(false);
  }

  const displayMods = formatModifiers(pending ? pending.modifierKeys : binding.modifierKeys);
  const displayKey = formatKey(pending ? pending.keyCode : binding.keyCode);
  const displaySeq = pending ? pending.sequence : binding.sequence;

  return (
    <tr className="keybinding-row">
      <td className="keybinding-row__info">
        <div className="keybinding-row__label">{binding.label}</div>
        {binding.description && (
          <div className="keybinding-row__desc">{binding.description}</div>
        )}
      </td>
      <td className="keybinding-row__cell">
        {listening && !pending ? (
          <span className="keybinding-row__listening">Press a key combination…</span>
        ) : (
          <div className="keybinding-row__combo">
            <div className="keybinding-row__keys">
              {displayMods.map((sym) => (
                <kbd key={sym} className="hotkey-kbd">{sym}</kbd>
              ))}
              <kbd className="hotkey-kbd">{displayKey}</kbd>
              {displaySeq && <kbd className="hotkey-kbd">{formatKey(displaySeq)}</kbd>}
            </div>
            {pending ? (
              <>
                <button className="btn-primary" style={{ fontSize: 12, padding: "3px 10px" }} onClick={handleSave}>Save</button>
                <button className="btn-secondary" style={{ fontSize: 12, padding: "3px 10px" }} onClick={handleCancel}>Cancel</button>
              </>
            ) : (
              <>
                <button className="btn-icon keybinding-row__change-btn" title="Change shortcut" onClick={handleChange}>
                  <Pencil size={13} />
                </button>
                {onReset && (
                  <button className="btn-icon keybinding-row__change-btn" title="Reset to default" onClick={() => onReset(binding.command)}>
                    <RotateCcw size={13} />
                  </button>
                )}
              </>
            )}
          </div>
        )}
      </td>
    </tr>
  );
}
