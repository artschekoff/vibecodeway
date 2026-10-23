import { useCallback, useEffect, useRef, useState } from "react";
import { useVimMode } from "./vimMode";

type VimMode = {
  dispose: () => void;
  on?: (event: string, handler: (ev: { mode: string }) => void) => void;
};

/**
 * Binds monaco-vim to any Monaco editor, driven by the app-wide vim setting.
 * Both the prose boxes (ProseEditor) and the code editor (DiffEditorTab) use
 * this, so they share one keymap, one status line and one on/off switch.
 *
 * Usage: call `attach(editor)` from Monaco's `onMount`, and render
 * `<VimStatusBar />` wherever the ex/command line belongs.
 */
export function useMonacoVim(opts?: {
  // Prose boxes start in INSERT mode: people type (or dictate) straight into
  // them, and a NORMAL-mode start silently eats the first characters as vim
  // commands until an `i`/`a` happens to land. Code editors keep NORMAL.
  startInInsert?: boolean;
}) {
  const enabled = useVimMode();
  const startInInsert = opts?.startInInsert ?? false;
  const editorRef = useRef<unknown>(null);
  const statusRef = useRef<HTMLDivElement | null>(null);
  const modeRef = useRef<VimMode | null>(null);
  // The current vim mode ("normal" | "insert" | "visual" | "replace"), kept
  // for call sites that bind keys differently per mode (Enter submits a note
  // in NORMAL, inserts a newline in INSERT).
  const vimState = useRef("normal");
  // Bumped when the editor mounts, so the binding effect re-runs once the
  // instance exists (onMount fires after the first render).
  const [mounted, setMounted] = useState(0);

  const attach = useCallback((editor: unknown) => {
    editorRef.current = editor;
    setMounted((n) => n + 1);
  }, []);

  useEffect(() => {
    if (!enabled || !editorRef.current || !statusRef.current) return;
    let cancelled = false;
    // vite.config.ts aliases "monaco-vim" to its ESM build — the default
    // ("browser") resolution serves a UMD bundle whose exports land under
    // `default` in production, silently breaking vim in the packaged app.
    void import("monaco-vim")
      .then((mod) => {
        const initVimMode = (mod.initVimMode ??
          (mod as { default?: { initVimMode?: unknown } }).default?.initVimMode) as
          | ((editor: unknown, status: HTMLElement) => VimMode)
          | undefined;
        if (!initVimMode) {
          console.error("monaco-vim: initVimMode export not found", Object.keys(mod));
          return;
        }
        if (cancelled || !editorRef.current || !statusRef.current) return;
        modeRef.current = initVimMode(editorRef.current, statusRef.current);
        modeRef.current.on?.("vim-mode-change", (ev) => { vimState.current = ev.mode; });
        if (startInInsert) {
          // VimMode.Vim is the CodeMirror vim API; the adapter instance stands
          // in for `cm`. Feeding "i" flips the fresh NORMAL state to INSERT.
          const VimModeCls = (mod.VimMode ??
            (mod as { default?: { VimMode?: unknown } }).default?.VimMode) as
            | { Vim?: { handleKey?: (cm: unknown, key: string) => void } }
            | undefined;
          VimModeCls?.Vim?.handleKey?.(modeRef.current, "i");
        }
      })
      .catch((err) => console.error("monaco-vim failed to load:", err));
    // Disposing restores the editor's normal keybindings, so flipping the
    // setting off takes effect without a remount.
    return () => {
      cancelled = true;
      modeRef.current?.dispose();
      modeRef.current = null;
    };
  }, [enabled, mounted, startInInsert]);

  const VimStatusBar = useCallback(
    () => (
      <div
        ref={statusRef}
        data-testid="vim-status"
        style={{
          display: enabled ? "block" : "none",
          fontFamily: "ui-monospace, monospace",
          fontSize: 11,
          color: "var(--text-muted)",
          padding: "2px 6px",
          borderTop: "1px solid var(--border)",
          minHeight: 18,
          flexShrink: 0,
        }}
      />
    ),
    [enabled],
  );

  const getVimMode = useCallback(() => vimState.current, []);

  return { enabled, attach, VimStatusBar, getVimMode };
}
