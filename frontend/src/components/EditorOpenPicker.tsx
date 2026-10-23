import { useEffect, useState } from "react";
import { useHotkey } from "../hotkeys/HotkeyProvider";
import { useTabContext } from "../contexts/TabContext";
import { useOpenContextEditor } from "../lib/useOpenContextEditor";

/**
 * Global ⌘E: open the code editor for the current context (the active tab).
 * A ready context with 2+ worktrees shows a keyboard-navigable picker; one (or
 * zero) worktrees opens directly via the shared useOpenContextEditor flow.
 *
 * ponytail: the not-ready checkout case (branch selection) lives in the context
 * view; ⌘E only surfaces a hint for it rather than duplicating that UI.
 */
export function EditorOpenPicker() {
  const { tabs, activeTabId, openEditorTab } = useTabContext();
  const opener = useOpenContextEditor(openEditorTab);
  const [highlight, setHighlight] = useState(0);

  useHotkey("editor.open", () => {
    const tab = tabs.find((t) => t.id === activeTabId);
    if (tab?.contextId && tab.contextKey) {
      setHighlight(0);
      void opener.open(tab.contextId, tab.contextKey);
    }
  });

  const worktrees = opener.workspacePicker?.worktrees ?? [];

  useEffect(() => {
    if (!opener.workspacePicker) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        opener.cancel();
        return;
      }
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setHighlight((h) => (worktrees.length ? (h + 1) % worktrees.length : 0));
        return;
      }
      if (e.key === "ArrowUp") {
        e.preventDefault();
        setHighlight((h) => (worktrees.length ? (h - 1 + worktrees.length) % worktrees.length : 0));
        return;
      }
      if (e.key === "Enter") {
        e.preventDefault();
        const wt = worktrees[highlight];
        if (wt) void opener.confirmWorkspace(wt);
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [opener, worktrees, highlight]);

  if (opener.workspacePicker) {
    return (
      <div className="modal-overlay" onClick={() => opener.cancel()}>
        <div
          className="modal editor-open-picker"
          role="dialog"
          aria-modal="true"
          aria-label="Open editor — choose worktree"
          onClick={(e) => e.stopPropagation()}
        >
          <h2>Open editor</h2>
          <div className="editor-open-list">
            {worktrees.map((wt, i) => (
              <div
                key={`${wt.repoId}\u0000${wt.branch}`}
                className={"editor-open-row" + (i === highlight ? " editor-open-row--highlighted" : "")}
                onMouseEnter={() => setHighlight(i)}
                onClick={() => void opener.confirmWorkspace(wt)}
              >
                <span className="editor-open-repo">{wt.repoName}</span>
                <span className="editor-open-branch">{wt.branch}</span>
                <span className="editor-open-path">{wt.worktreePath || "not checked out yet"}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  if (opener.picker || opener.error) {
    return (
      <div className="modal-overlay" onClick={() => opener.cancel()}>
        <div
          className="modal editor-open-picker"
          role="dialog"
          aria-modal="true"
          aria-label="Open editor"
          onClick={(e) => e.stopPropagation()}
        >
          <h2>Open editor</h2>
          <p className="editor-open-hint">
            {opener.error ??
              "This context has no checked-out worktree yet. Open it from the context view to check out a branch."}
          </p>
          <div style={{ textAlign: "right" }}>
            <button className="btn-secondary" onClick={() => opener.cancel()}>Close</button>
          </div>
        </div>
      </div>
    );
  }

  return null;
}
