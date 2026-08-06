import { useEffect, useRef, useState } from "react";
import { DiffEditor } from "@monaco-editor/react";
import { History, ScrollText, Maximize2, Minimize2, ChevronUp, ChevronDown, Sparkles } from "lucide-react";
import { ReadRepoFile, ReadRepoFileBase, ReadRepoFileHead, SaveRepoFile, FileBlame, DiscardFile, ConfirmAction } from "../../../wailsjs/go/main/App";
import { setupMonaco } from "../../lib/monacoSetup";
import { useEditorSettings, toMonacoOptions } from "../../contexts/EditorSettingsContext";
import { EditorSettingsMenu } from "./EditorSettingsMenu";
import { FileHistoryModal } from "./FileHistoryModal";
import { computeHunkRevert, type LineChange } from "../../lib/hunkRevert";
import { languageFor } from "../../lib/languageFor";

type ContentWidget = {
  getId: () => string;
  getDomNode: () => HTMLElement;
  getPosition: () => { position: { lineNumber: number; column: number }; preference: number[] } | null;
};
type ModifiedEditor = {
  getValue: () => string;
  updateOptions: (o: Record<string, unknown>) => void;
  deltaDecorations: (old: string[], neu: unknown[]) => string[];
  revealLineInCenter?: (line: number) => void;
  setPosition?: (pos: { lineNumber: number; column: number }) => void;
  getSelection?: () => { startLineNumber: number; endLineNumber: number } | null;
  focus?: () => void;
  addContentWidget?: (w: ContentWidget) => void;
  removeContentWidget?: (w: ContentWidget) => void;
  executeEdits?: (source: string, edits: { range: unknown; text: string }[]) => boolean;
  onDidChangeModelContent?: (cb: () => void) => { dispose: () => void };
  getModel?: () => { getLineCount: () => number } | null;
};
type DiffEditorInstance = { getModifiedEditor: () => ModifiedEditor; getLineChanges?: () => LineChange[] | null };
// Minimal shape of the monaco namespace we use for blame decorations.
type MonacoNs = { Range: new (a: number, b: number, c: number, d: number) => unknown };

const iconBtn = (active: boolean): React.CSSProperties => ({
  display: "flex", alignItems: "center", background: "none", border: "none", cursor: "pointer",
  padding: 2, color: active ? "var(--accent, #2563eb)" : "var(--text-muted)",
});

export function DiffEditorTab({
  repoPath, path, onSaved, refreshSignal, onAskAi, baseMode = "branch",
}: { repoPath: string; path: string; onSaved: () => void; refreshSignal?: number; onAskAi?: (text: string) => void; baseMode?: "branch" | "head" }) {
  const [original, setOriginal] = useState<string | null>(null);
  const [modified, setModified] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [blameOn, setBlameOn] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [fullscreen, setFullscreen] = useState(false);
  const [localReload, setLocalReload] = useState(0);
  const editorRef = useRef<DiffEditorInstance | null>(null);
  const monacoRef = useRef<MonacoNs | null>(null);
  const blameIdsRef = useRef<string[]>([]);
  const changeIdxRef = useRef(-1);
  const { settings } = useEditorSettings();

  // Jump to the next/previous changed hunk in the file (cycles through the diff).
  const goToChange = (dir: 1 | -1) => {
    const diff = editorRef.current;
    const changes = diff?.getLineChanges?.() ?? [];
    if (!diff || changes.length === 0) return;
    changeIdxRef.current = (changeIdxRef.current + dir + changes.length) % changes.length;
    const ch = changes[changeIdxRef.current];
    const line = ch.modifiedStartLineNumber || ch.modifiedEndLineNumber || 1;
    const me = diff.getModifiedEditor();
    me.revealLineInCenter?.(line);
    me.setPosition?.({ lineNumber: line, column: 1 });
    me.focus?.();
  };

  const hunkWidgetsRef = useRef<ContentWidget[]>([]);

  const clearHunkWidgets = () => {
    const me = editorRef.current?.getModifiedEditor();
    if (me?.removeContentWidget) hunkWidgetsRef.current.forEach((w) => me.removeContentWidget!(w));
    hunkWidgetsRef.current = [];
  };

  const rejectHunk = (change: LineChange) => {
    const me = editorRef.current?.getModifiedEditor();
    const monaco = monacoRef.current;
    if (!me || !monaco || original === null || !me.executeEdits) return;
    const rev = computeHunkRevert(original.split("\n"), change);
    const lineCount = me.getModel?.()?.getLineCount?.() ?? rev.removeToLine + 1;
    // Build a whole-line replacement range on the modified model.
    let range: unknown;
    let text: string;
    if (rev.removeToLine >= rev.removeFromLine) {
      const atEof = rev.removeToLine >= lineCount;
      if (!atEof) {
        range = new monaco.Range(rev.removeFromLine, 1, rev.removeToLine + 1, 1);
        text = rev.insertLines.length ? rev.insertLines.join("\n") + "\n" : "";
      } else if (rev.removeFromLine > 1) {
        // consume the newline that precedes the first removed line
        range = new monaco.Range(rev.removeFromLine - 1, Number.MAX_SAFE_INTEGER, rev.removeToLine, Number.MAX_SAFE_INTEGER);
        text = rev.insertLines.length ? "\n" + rev.insertLines.join("\n") : "";
      } else {
        // hunk starts at line 1 AND reaches EOF: replace the whole content range from the very start
        range = new monaco.Range(1, 1, rev.removeToLine, Number.MAX_SAFE_INTEGER);
        text = rev.insertLines.join("\n");
      }
    } else {
      // Pure insertion at line rev.removeFromLine.
      if (rev.removeFromLine > lineCount) {
        // Deletion anchored at EOF: append AFTER the last line with a leading
        // newline (no trailing one), so the base lines aren't fused onto it.
        range = new monaco.Range(lineCount, Number.MAX_SAFE_INTEGER, lineCount, Number.MAX_SAFE_INTEGER);
        text = "\n" + rev.insertLines.join("\n");
      } else {
        range = new monaco.Range(rev.removeFromLine, 1, rev.removeFromLine, 1);
        text = rev.insertLines.join("\n") + "\n";
      }
    }
    me.executeEdits("reject-hunk", [{ range, text }]);
    void handleSave();
  };

  const buildHunkWidgets = () => {
    const me = editorRef.current?.getModifiedEditor();
    const changes = editorRef.current?.getLineChanges?.() ?? [];
    if (!me || !me.addContentWidget) return;
    clearHunkWidgets();
    changes.forEach((change, i) => {
      const line = change.modifiedStartLineNumber || 1;
      // One segmented pill split into two halves (Keep | Revert). Styled inline so
      // the row layout can't be defeated by a stale CSS bundle or a Monaco content-
      // widget stylesheet — the earlier .hunk-actions flex rule wasn't taking effect.
      const dom = document.createElement("div");
      dom.className = "hunk-actions";
      dom.style.cssText =
        "display:inline-flex;flex-direction:row;align-items:stretch;border:1px solid var(--border);" +
        "border-radius:5px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,0.3);" +
        "transform:translateY(-2px);font-size:10px;line-height:1;z-index:5;";
      const half =
        "border:0;background:var(--surface2);padding:3px 9px;cursor:pointer;font-size:10px;" +
        "line-height:1;white-space:nowrap;font-weight:600;font-family:inherit;";
      const accept = document.createElement("button");
      accept.textContent = "✓ Keep";
      accept.title = "Accept this change (keep it)";
      accept.style.cssText = half + "color:#4ade80;";
      const reject = document.createElement("button");
      reject.textContent = "✕ Revert";
      reject.title = "Reject this change (restore the base version)";
      reject.style.cssText = half + "color:#f87171;border-left:1px solid var(--border);";
      // Inline hover (a :hover rule can't reach an inline-styled node reliably here).
      accept.onmouseenter = () => { accept.style.background = "rgba(74,222,128,0.14)"; };
      accept.onmouseleave = () => { accept.style.background = "var(--surface2)"; };
      reject.onmouseenter = () => { reject.style.background = "rgba(248,113,113,0.14)"; };
      reject.onmouseleave = () => { reject.style.background = "var(--surface2)"; };
      dom.appendChild(accept);
      dom.appendChild(reject);
      const widget: ContentWidget = {
        getId: () => `hunk-action-${i}`,
        getDomNode: () => dom,
        getPosition: () => ({ position: { lineNumber: line, column: 1 }, preference: [1, 0] }),
      };
      accept.onclick = () => { me.removeContentWidget?.(widget); };
      reject.onclick = () => rejectHunk(change);
      me.addContentWidget!(widget);
      hunkWidgetsRef.current.push(widget);
    });
  };

  const buildHunkWidgetsRef = useRef(buildHunkWidgets);
  buildHunkWidgetsRef.current = buildHunkWidgets;

  const { theme, options } = toMonacoOptions(settings);

  useEffect(() => { setupMonaco(); }, []);

  // Esc exits fullscreen — but let the history modal consume Esc first.
  useEffect(() => {
    if (!fullscreen) return;
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape" && !historyOpen) setFullscreen(false); };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [fullscreen, historyOpen]);

  // Toggle inline blame: glyph-margin decorations on the modified editor, each
  // hovering to its commit/author/date/summary. Best-effort — needs the editor
  // mounted (true once the user can click the toggle).
  useEffect(() => {
    const ed = editorRef.current?.getModifiedEditor();
    const monaco = monacoRef.current;
    if (!ed || !monaco) return;
    if (!blameOn) {
      blameIdsRef.current = ed.deltaDecorations(blameIdsRef.current, []);
      ed.updateOptions({ glyphMargin: false });
      return;
    }
    ed.updateOptions({ glyphMargin: true });
    let active = true;
    void FileBlame(repoPath, path).then((lines) => {
      const me = editorRef.current?.getModifiedEditor();
      if (!active || !me) return;
      const decos = (lines ?? []).map((l) => ({
        range: new monaco.Range(l.line, 1, l.line, 1),
        options: {
          glyphMarginClassName: "blame-glyph",
          glyphMarginHoverMessage: { value: `\`${l.commit}\` ${l.author} · ${l.date} · ${l.summary}` },
        },
      }));
      blameIdsRef.current = me.deltaDecorations(blameIdsRef.current, decos);
    });
    return () => { active = false; };
  }, [blameOn, repoPath, path, refreshSignal, original, modified]);

  useEffect(() => {
    let active = true;
    // "head" (Staged/Local/file-tree) diffs against HEAD so only uncommitted
    // edits show; "branch" (Branch Changes) diffs against the merge-base for the
    // full branch/PR change.
    const readBase = baseMode === "head" ? ReadRepoFileHead : ReadRepoFileBase;
    void Promise.all([readBase(repoPath, path), ReadRepoFile(repoPath, path)])
      .then(([head, work]) => {
        if (!active) return;
        setOriginal(head);
        setModified(work);
      })
      .catch((e) => { if (active) setError(e instanceof Error ? e.message : String(e)); });
    return () => { active = false; };
  }, [repoPath, path, refreshSignal, localReload, baseMode]);

  // Rebuild the per-hunk accept/reject widgets whenever the diff content reloads.
  useEffect(() => { const t = setTimeout(buildHunkWidgets, 50); return () => clearTimeout(t); }, [original, modified, refreshSignal]);

  useEffect(() => () => clearHunkWidgets(), []);

  // Build an AI prompt referencing the selected file/line/segment and hand it to
  // the parent, which types it into the editor's terminal (for the user to send).
  const askAI = () => {
    if (!onAskAi || modified === null) return;
    const sel = editorRef.current?.getModifiedEditor().getSelection?.();
    const a = sel?.startLineNumber ?? 1;
    const b = sel?.endLineNumber ?? a;
    const segment = modified.split("\n").slice(a - 1, b).join("\n");
    const loc = a === b ? `line ${a}` : `lines ${a}-${b}`;
    onAskAi(`Regarding \`${path}\` (${loc}):\n\`\`\`\n${segment}\n\`\`\`\n`);
  };

  const handleSave = async () => {
    if (!editorRef.current) return;
    setSaving(true);
    setError(null);
    try {
      const value = editorRef.current.getModifiedEditor().getValue();
      await SaveRepoFile(repoPath, path, value);
      onSaved();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  };

  const hasChanges = original !== null && modified !== null && original !== modified;

  const acceptAll = () => { clearHunkWidgets(); }; // keep everything; just dismiss the hunk buttons

  const revertAll = async () => {
    if (!(await ConfirmAction("Revert changes", `Revert ALL changes in ${path} to the base version? This cannot be undone.`))) return;
    setError(null);
    try {
      await DiscardFile(repoPath, path);
      onSaved();                 // let the changed-files panel refresh
      setLocalReload((n) => n + 1); // re-read base+working so the diff clears
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  };

  const rootStyle: React.CSSProperties = fullscreen
    ? { position: "fixed", inset: 0, zIndex: 90, background: "var(--bg)", display: "flex", flexDirection: "column" }
    : { display: "flex", flexDirection: "column", height: "100%" };

  return (
    <div style={rootStyle}>
      <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "4px 8px", borderBottom: "1px solid var(--border)" }}>
        <span style={{ fontSize: 11 }}>{path}</span>
        <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 6 }}>
          <button aria-label="previous change" title="Previous change" style={iconBtn(false)} onClick={() => goToChange(-1)}>
            <ChevronUp size={13} />
          </button>
          <button aria-label="next change" title="Next change" style={iconBtn(false)} onClick={() => goToChange(1)}>
            <ChevronDown size={13} />
          </button>
          {onAskAi && (
            <button aria-label="ask AI about selection" title="Ask AI (inserts a prompt about the selected lines into the terminal)" style={iconBtn(false)} onClick={askAI}>
              <Sparkles size={13} />
            </button>
          )}
          <button aria-label="toggle blame" title="Toggle blame" style={iconBtn(blameOn)} onClick={() => setBlameOn((b) => !b)}>
            <ScrollText size={13} />
          </button>
          <button aria-label="file history" title="File history" style={iconBtn(false)} onClick={() => setHistoryOpen(true)}>
            <History size={13} />
          </button>
          <button
            aria-label={fullscreen ? "exit fullscreen" : "fullscreen editor"}
            title={fullscreen ? "Exit fullscreen (Esc)" : "Fullscreen editor"}
            style={iconBtn(fullscreen)}
            onClick={() => setFullscreen((f) => !f)}
          >
            {fullscreen ? <Minimize2 size={13} /> : <Maximize2 size={13} />}
          </button>
          {hasChanges && (
            <button className="btn-secondary" style={{ fontSize: 11, padding: "2px 8px" }}
              aria-label="accept all changes" title="Accept all changes (keep them)" onClick={acceptAll}>
              Accept all
            </button>
          )}
          {hasChanges && (
            <button className="btn-secondary" style={{ fontSize: 11, padding: "2px 8px" }}
              aria-label="revert all changes" title="Revert all changes to the base version" onClick={() => void revertAll()}>
              Revert all
            </button>
          )}
          <EditorSettingsMenu />
          <button className="btn-primary" style={{ fontSize: 11, padding: "2px 10px" }}
            disabled={saving} onClick={() => void handleSave()}>
            {saving ? "Saving…" : "Save"}
          </button>
        </div>
      </div>
      {error && <div style={{ fontSize: 11, color: "#dc2626", padding: "2px 8px" }}>{error}</div>}
      <div style={{ flex: 1, minHeight: 0 }}>
        {original !== null && modified !== null && (
          <DiffEditor
            original={original}
            modified={modified}
            language={languageFor(path)}
            theme={theme}
            options={options}
            onMount={(editor, monaco) => {
              editorRef.current = editor as unknown as DiffEditorInstance;
              monacoRef.current = monaco as unknown as MonacoNs;
              const me = editorRef.current.getModifiedEditor();
              // Rebuild the accept/reject widgets whenever the modified text changes
              // (Monaco recomputes the diff), plus once now for the initial diff.
              me.onDidChangeModelContent?.(() => setTimeout(() => buildHunkWidgetsRef.current(), 0));
              setTimeout(buildHunkWidgets, 0);
            }}
          />
        )}
      </div>
      {historyOpen && <FileHistoryModal repoPath={repoPath} path={path} onClose={() => setHistoryOpen(false)} />}
    </div>
  );
}
