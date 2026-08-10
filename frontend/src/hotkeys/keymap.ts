import { parseChord, serializeChord, type Chord } from "./chord";
import type { Command } from "./registry";
import type { Keybinding } from "../types";

export interface Binding {
  command: Command;
  chord: Chord;
  sequence?: string;
}

/**
 * Resolve a DB Keybinding override's neutral modifier tokens into a Chord.
 * "cmd" -> mod, "ctrl" -> ctrl, "shift"/"alt" -> shift/alt. keyCode -> key.
 */
function chordFromOverride(override: Pick<Keybinding, "modifierKeys" | "keyCode">): Chord {
  const tokens = (override.modifierKeys || "")
    .split(",")
    .map((t) => t.trim().toLowerCase())
    .filter(Boolean);
  return {
    mod: tokens.includes("cmd"),
    ctrl: tokens.includes("ctrl"),
    shift: tokens.includes("shift"),
    alt: tokens.includes("alt"),
    key: (override.keyCode || "").toLowerCase(),
  };
}

/**
 * Build the runtime keymap from the static command registry plus any DB
 * overrides. `lead` groups bindings (bare + sequenced) by the serialized
 * lead chord so the dispatcher can find all children of a leader press in
 * one lookup. `byId` gives O(1) access to a single command's resolved
 * binding (used for chord hints / help listings).
 */
export function buildKeymap(
  commands: Command[],
  overrides: Keybinding[]
): { lead: Map<string, Binding[]>; byId: Map<string, Binding> } {
  const overrideByCommand = new Map<string, Keybinding>();
  for (const o of overrides) overrideByCommand.set(o.command, o);

  const byId = new Map<string, Binding>();
  const lead = new Map<string, Binding[]>();

  for (const command of commands) {
    const override = overrideByCommand.get(command.id);
    const chord = override ? chordFromOverride(override) : parseChord(command.defaultChord);
    const sequence = override ? (override.sequence || undefined) : command.sequence;

    const binding: Binding = { command, chord, sequence };
    byId.set(command.id, binding);

    const leadKey = serializeChord(chord);
    const bucket = lead.get(leadKey);
    if (bucket) bucket.push(binding);
    else lead.set(leadKey, [binding]);
  }

  return { lead, byId };
}
