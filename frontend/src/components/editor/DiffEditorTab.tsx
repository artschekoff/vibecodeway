import { useEffect, useRef, useState } from "react";
import { DiffEditor } from "@monaco-editor/react";
import { History, ScrollText, Maximize2, Minimize2 } from "lucide-react";
import { ReadRepoFile, ReadRepoFileBase, SaveRepoFile, FileBlame } from "../../../wailsjs/go/main/App";
import { setupMonaco } from "../../lib/monacoSetup";
import { useEditorSettings, toMonacoOptions } from "../../contexts/EditorSettingsContext";
import { EditorSettingsMenu } from "./EditorSettingsMenu";
import { FileHistoryModal } from "./FileHistoryModal";

type ModifiedEditor = {
  getValue: () => string;
  updateOptions: (o: Record<string, unknown>) => void;
  deltaDecorations: (old: string[], neu: unknown[]) => string[];
};
type DiffEditorInstance = { getModifiedEditor: () => ModifiedEditor };
// Minimal shape of the monaco namespace we use for blame decorations.
type MonacoNs = { Range: new (a: number, b: number, c: number, d: number) => unknown };

const iconBtn = (active: boolean): React.CSSProperties => ({
  display: "flex", alignItems: "center", background: "none", border: "none", cursor: "pointer",
  padding: 2, color: active ? "var(--accent, #2563eb)" : "var(--text-muted)",
});

export function languageFor(path: string): string {
  const ext = path.split(".").pop() ?? "";
  const map: Record<string, string> = {
    ts: "typescript", tsx: "typescript", js: "javascript", jsx: "javascript",
    go: "go", sql: "sql", json: "json", css: "css", html: "html", md: "markdown",
    py: "python", sh: "shell", yml: "yaml", yaml: "yaml",
  };
  return map[ext] ?? "plaintext";
}

export function DiffEditorTab({
  repoPath, path, onSaved, refreshSignal,
}: { repoPath: string; path: string; onSaved: () => void; refreshSignal?: number }) {
  const [original, setOriginal] = useState<string | null>(null);
  const [modified, setModified] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [blameOn, setBlameOn] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [fullscreen, setFullscreen] = useState(false);
  const editorRef = useRef<DiffEditorInstance | null>(null);
  const monacoRef = useRef<MonacoNs | null>(null);
  const blameIdsRef = useRef<string[]>([]);
  const { settings } = useEditorSettings();
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
    void Promise.all([ReadRepoFileBase(repoPath, path), ReadRepoFile(repoPath, path)])
      .then(([head, work]) => {
        if (!active) return;
        setOriginal(head);
        setModified(work);
      })
      .catch((e) => { if (active) setError(e instanceof Error ? e.message : String(e)); });
    return () => { active = false; };
  }, [repoPath, path, refreshSignal]);

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

  const rootStyle: React.CSSProperties = fullscreen
    ? { position: "fixed", inset: 0, zIndex: 90, background: "var(--bg)", display: "flex", flexDirection: "column" }
    : { display: "flex", flexDirection: "column", height: "100%" };

  return (
    <div style={rootStyle}>
      <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "4px 8px", borderBottom: "1px solid var(--border)" }}>
        <span style={{ fontSize: 11 }}>{path}</span>
        <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 6 }}>
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
            }}
          />
        )}
      </div>
      {historyOpen && <FileHistoryModal repoPath={repoPath} path={path} onClose={() => setHistoryOpen(false)} />}
    </div>
  );
}
