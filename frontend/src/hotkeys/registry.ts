import { parseChord, formatChord, type Platform } from "./chord";
import type { Scope } from "./scopes";

export interface Command {
  id: string; object: string; action: string; label: string; description: string;
  scope: Scope; defaultChord: string; sequence?: string; defaultAction?: boolean;
}

const cap = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);

export const COMMANDS: Command[] = [
  // --- global ---
  { id: "search.global", object: "search", action: "global", label: "Global search",
    description: "Search contexts across all organizations", scope: "global", defaultChord: "mod+f" },
  { id: "editor.open", object: "editor", action: "open", label: "Open editor",
    description: "Open the code editor for the current context", scope: "global", defaultChord: "mod+e" },
  { id: "help.open", object: "help", action: "open", label: "Keyboard help",
    description: "Show all keyboard shortcuts", scope: "global", defaultChord: "mod+h" },
  { id: "context.copyName", object: "context", action: "copyName", label: "Copy context name",
    description: "Copy the current context key to the clipboard", scope: "global", defaultChord: "mod+c", sequence: "n" },
  { id: "pr.open", object: "pr", action: "open", label: "Open pull request",
    description: "Open the context's pull request in the browser", scope: "global", defaultChord: "mod+g", sequence: "p" },
  { id: "pr.reload", object: "pr", action: "reload", label: "Reload pull requests",
    description: "Re-sync the pull-request list", scope: "global", defaultChord: "mod+p", sequence: "r", defaultAction: true },
  { id: "tabs.next", object: "tabs", action: "next", label: "Next open context",
    description: "Switch to the next open context tab", scope: "global", defaultChord: "mod+]" },
  { id: "tabs.prev", object: "tabs", action: "prev", label: "Previous open context",
    description: "Switch to the previous open context tab", scope: "global", defaultChord: "mod+[" },
  // --- editor zone focus (vim-like leader: ⌘E then h/j/k/l) ---
  // Shares the ⌘E lead with the bare "editor.open" command: ⌘E alone still
  // opens the editor (fall-through default), ⌘E + h/j/k/l moves zone focus.
  { id: "editor.focusUp", object: "editor", action: "focusUp", label: "Focus zone up",
    description: "Move focus to the zone above", scope: "editor", defaultChord: "mod+e", sequence: "k" },
  { id: "editor.focusDown", object: "editor", action: "focusDown", label: "Focus zone down",
    description: "Move focus to the zone below", scope: "editor", defaultChord: "mod+e", sequence: "j" },
  { id: "editor.focusLeft", object: "editor", action: "focusLeft", label: "Focus zone left",
    description: "Move focus to the zone to the left", scope: "editor", defaultChord: "mod+e", sequence: "h" },
  { id: "editor.focusRight", object: "editor", action: "focusRight", label: "Focus zone right",
    description: "Move focus to the zone to the right", scope: "editor", defaultChord: "mod+e", sequence: "l" },
  // --- sidebar tabs (scope: editor — active when editor/sidebar focused, NOT terminal) ---
  { id: "sidebar.next", object: "sidebar", action: "next", label: "Next sidebar tab",
    description: "Switch FILES / GIT (forward)", scope: "editor", defaultChord: "mod+shift+]" },
  { id: "sidebar.prev", object: "sidebar", action: "prev", label: "Previous sidebar tab",
    description: "Switch FILES / GIT (back)", scope: "editor", defaultChord: "mod+shift+[" },
  // --- files ---
  { id: "files.reload", object: "files", action: "reload", label: "Reload files",
    description: "Reload the file list from disk/git", scope: "files", defaultChord: "mod+r" },
  { id: "files.focusSearch", object: "files", action: "focusSearch", label: "Focus file search",
    description: "Move focus to the file filter input", scope: "files", defaultChord: "mod+f", sequence: "f" },
  // --- terminal (scope: terminal) ---
  { id: "terminal.new", object: "terminal", action: "new", label: "New terminal",
    description: "Open a new terminal (same as +)", scope: "terminal", defaultChord: "mod+t", defaultAction: true },
  { id: "terminal.here", object: "terminal", action: "here", label: "Terminal here",
    description: "Open a terminal at the selected path", scope: "terminal", defaultChord: "mod+t", sequence: "h" },
  { id: "terminal.insert", object: "terminal", action: "insert", label: "Insert command",
    description: "Open the insert-command menu", scope: "terminal", defaultChord: "mod+t", sequence: "i" },
  { id: "terminal.next", object: "terminal", action: "next", label: "Next terminal",
    description: "Switch to the next terminal", scope: "terminal", defaultChord: "mod+shift+]" },
  { id: "terminal.prev", object: "terminal", action: "prev", label: "Previous terminal",
    description: "Switch to the previous terminal", scope: "terminal", defaultChord: "mod+shift+[" },
].map((c) => ({ ...c })) as Command[];

export function commandById(id: string): Command | undefined {
  return COMMANDS.find((c) => c.id === id);
}

export function commandLabel(c: Command, platform: Platform): string {
  const chord = c.defaultChord + (c.sequence ? ` ${c.sequence}` : "");
  const lead = chord.split(" ").map((p) => formatChord(parseChord(p), platform)).join(" ");
  return `${lead} · ${cap(c.object)} · ${cap(c.action)}`;
}
