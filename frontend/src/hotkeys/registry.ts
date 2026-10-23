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
    description: "Search contexts across all organizations", scope: "global", defaultChord: "mod+shift+f" },
  { id: "context.filterItems", object: "context", action: "filterItems", label: "Filter context items",
    description: "Focus the item filter of the open context", scope: "global", defaultChord: "mod+f" },
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
  { id: "reviews.refresh", object: "reviews", action: "refresh", label: "Refresh code reviews",
    description: "Re-fetch every pending code review on the Code Reviews tab", scope: "global", defaultChord: "mod+r" },
  { id: "board.open", object: "board", action: "open", label: "Open My Board",
    description: "Open the My Board tab", scope: "global", defaultChord: "mod+shift+m" },
  { id: "ai.ask", object: "ai", action: "ask", label: "Open Ask AI",
    description: "Open the Ask AI tab", scope: "global", defaultChord: "mod+shift+a" },
  { id: "context.refresh", object: "context", action: "refresh", label: "Refresh context",
    description: "Refresh the active context from external sources", scope: "global", defaultChord: "mod+shift+r" },
  // --- context detail topbar (one chord per button) ---
  { id: "context.back", object: "context", action: "back", label: "Back",
    description: "Go back from the context detail view", scope: "global", defaultChord: "mod+shift+b" },
  { id: "context.markAllViewed", object: "context", action: "markAllViewed", label: "Mark all viewed",
    description: "Mark every item in the context as viewed", scope: "global", defaultChord: "mod+shift+v" },
  { id: "context.assignAuthors", object: "context", action: "assignAuthors", label: "Assign authors",
    description: "Open the unknown-authors assignment dialog", scope: "global", defaultChord: "mod+shift+p" },
  { id: "context.edit", object: "context", action: "edit", label: "Edit context",
    description: "Open the Edit Context form", scope: "global", defaultChord: "mod+shift+e" },
  { id: "context.solveAI", object: "context", action: "solveAI", label: "Solve with AI",
    description: "Open the Solve with AI panel", scope: "global", defaultChord: "mod+shift+s" },
  { id: "tour.start", object: "tour", action: "start", label: "Start code tour",
    description: "Start (or resume) the code tour in the editor", scope: "editor", defaultChord: "mod+shift+u" },
  { id: "symbols.search", object: "symbols", action: "search", label: "Search symbols",
    description: "Fuzzy-search functions, classes and variables in the active repo", scope: "editor", defaultChord: "mod+shift+o" },
  { id: "files.quickOpen", object: "files", action: "quickOpen", label: "Go to file",
    description: "Fuzzy-search file names in the active repo", scope: "editor", defaultChord: "mod+o" },
  { id: "search.code", object: "search", action: "code", label: "Search everything",
    description: "Search files and symbols in the active repo at once", scope: "editor", defaultChord: "mod+f" },
  { id: "tabs.next", object: "tabs", action: "next", label: "Next open context",
    description: "Switch to the next open context tab", scope: "global", defaultChord: "mod+shift+]" },
  { id: "tabs.prev", object: "tabs", action: "prev", label: "Previous open context",
    description: "Switch to the previous open context tab", scope: "global", defaultChord: "mod+shift+[" },
  { id: "tabs.close", object: "tabs", action: "close", label: "Close context tab",
    description: "Close the active context tab", scope: "global", defaultChord: "mod+shift+w" },
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
  // ⌘B is a leader: ⌘B F shows FILES, ⌘B G shows GIT (direct select, no cycling).
  { id: "sidebar.files", object: "sidebar", action: "files", label: "Show FILES",
    description: "Show the FILES sidebar tab", scope: "editor", defaultChord: "mod+b", sequence: "f" },
  { id: "sidebar.git", object: "sidebar", action: "git", label: "Show GIT",
    description: "Show the GIT sidebar tab", scope: "editor", defaultChord: "mod+b", sequence: "g" },
  // --- files ---
  { id: "files.reload", object: "files", action: "reload", label: "Reload files",
    description: "Reload the file list from disk/git", scope: "files", defaultChord: "mod+r" },
  { id: "files.focusSearch", object: "files", action: "focusSearch", label: "Focus file search",
    description: "Move focus to the file filter input", scope: "files", defaultChord: "mod+f", sequence: "f" },
  // --- terminal ---
  // new / here / insert are scope:"global" so ⌘T works anywhere in the app (the
  // terminal is rarely the focused element when you want to open one). `object`
  // stays "terminal" so they still group under TERMINAL in the help/settings UI.
  // next / prev stay scope:"terminal" — cycling only makes sense with the panel focused.
  //
  // Each has its OWN chord — deliberately not a ⌘T leader. "here" and "insert"
  // used to hang off ⌘T as sequenced children, which forced every ⌘T through the
  // 600ms leader window before the terminal appeared; opening a terminal is the
  // hot path and has to land on the keypress.
  { id: "terminal.new", object: "terminal", action: "new", label: "New terminal",
    description: "Open a new terminal (same as +)", scope: "global", defaultChord: "mod+t" },
  { id: "terminal.here", object: "terminal", action: "here", label: "Terminal here",
    description: "Open a terminal at the selected path", scope: "global", defaultChord: "mod+shift+t" },
  { id: "terminal.insert", object: "terminal", action: "insert", label: "Insert command",
    description: "Open the insert-command menu", scope: "global", defaultChord: "mod+i" },
  // "global", not "terminal": scoping these to DOM focus made them dead whenever
  // focus sat anywhere but inside xterm — including right after clicking a
  // terminal tab, which is not focusable. Same bug ⌘T had. ⌘]/⌘[ are bound to
  // nothing else (context tabs moved to ⌘⇧]/⌘⇧[ in migration 107), so there is
  // no chord to lose by widening the scope.
  { id: "terminal.next", object: "terminal", action: "next", label: "Next terminal",
    description: "Switch to the next terminal", scope: "global", defaultChord: "mod+]" },
  { id: "terminal.prev", object: "terminal", action: "prev", label: "Previous terminal",
    description: "Switch to the previous terminal", scope: "global", defaultChord: "mod+[" },
  { id: "terminal.jumpWaiting", object: "terminal", action: "jumpWaiting", label: "Jump to waiting terminal",
    description: "Go to the next terminal whose agent is waiting for input", scope: "global", defaultChord: "mod+shift+j" },
].map((c) => ({ ...c })) as Command[];

export function commandById(id: string): Command | undefined {
  return COMMANDS.find((c) => c.id === id);
}

export function commandLabel(c: Command, platform: Platform): string {
  const chord = c.defaultChord + (c.sequence ? ` ${c.sequence}` : "");
  const lead = chord.split(" ").map((p) => formatChord(parseChord(p), platform)).join(" ");
  return `${lead} · ${cap(c.object)} · ${cap(c.action)}`;
}
