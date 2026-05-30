import { useEffect, useState } from "react";
import { Pencil } from "lucide-react";
import type { Keybinding } from "../types";

interface Props {
  binding: Keybinding;
  onSave: (command: string, modifierKeys: string, keyCode: string) => void;
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
  return modifierKeys.split(",").map((m) => MOD_SYMBOLS[m.trim()] ?? m.toUpperCase());
}

function formatKey(keyCode: string): string {
  return KEY_SYMBOLS[keyCode] ?? keyCode.toUpperCase();
}

interface PendingCombo {
  modifierKeys: string; // comma-separated
  keyCode: string;
}

function captureCombo(e: KeyboardEvent): PendingCombo | null {
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

export default function KeybindingRow({ binding, onSave }: Props) {
  const [listening, setListening] = useState(false);
  const [pending, setPending] = useState<PendingCombo | null>(null);

  useEffect(() => {
    if (!listening) return;

    const handler = (e: KeyboardEvent) => {
      e.preventDefault();
      if (e.key === "Escape") {
        setListening(false);
        setPending(null);
        return;
      }
      const combo = captureCombo(e);
      if (combo) {
        setPending(combo);
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
    onSave(binding.command, pending.modifierKeys, pending.keyCode);
    setPending(null);
  }

  function handleCancel() {
    setPending(null);
    setListening(false);
  }

  const displayMods = formatModifiers(pending ? pending.modifierKeys : binding.modifierKeys);
  const displayKey = formatKey(pending ? pending.keyCode : binding.keyCode);

  return (
    <tr className="keybinding-row">
      <td className="keybinding-row__info">
        <div className="keybinding-row__label">{binding.label}</div>
        {binding.description && (
          <div className="keybinding-row__desc">{binding.description}</div>
        )}
      </td>
      <td className="keybinding-row__cell">
        {listening ? (
          <span className="keybinding-row__listening">Press a key combination…</span>
        ) : (
          <div className="keybinding-row__combo">
            <div className="keybinding-row__keys">
              {displayMods.map((sym) => (
                <kbd key={sym} className="hotkey-kbd">{sym}</kbd>
              ))}
              <kbd className="hotkey-kbd">{displayKey}</kbd>
            </div>
            {pending ? (
              <>
                <button className="btn-primary" style={{ fontSize: 12, padding: "3px 10px" }} onClick={handleSave}>Save</button>
                <button className="btn-secondary" style={{ fontSize: 12, padding: "3px 10px" }} onClick={handleCancel}>Cancel</button>
              </>
            ) : (
              <button className="btn-icon keybinding-row__change-btn" title="Change shortcut" onClick={handleChange}>
                <Pencil size={13} />
              </button>
            )}
          </div>
        )}
      </td>
    </tr>
  );
}
