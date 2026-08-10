import { useChordHint } from "../hotkeys/HotkeyProvider";

/** Inline live chord hint for an action label, e.g. " (⌘E)". Reflects rebinds. */
export function ChordHint({ id }: { id: string }) {
  const chord = useChordHint(id);
  if (!chord) return null;
  return <span className="chord-hint"> ({chord})</span>;
}

export default ChordHint;
