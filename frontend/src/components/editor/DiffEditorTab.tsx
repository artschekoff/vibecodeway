import { useEffect, useRef, useState } from "react";
import { DiffEditor } from "@monaco-editor/react";
import { ReadRepoFile, ReadRepoFileHead, SaveRepoFile } from "../../../wailsjs/go/main/App";
import { setupMonaco } from "../../lib/monacoSetup";

type ModifiedGetter = { getModifiedEditor: () => { getValue: () => string } };

function languageFor(path: string): string {
  const ext = path.split(".").pop() ?? "";
  const map: Record<string, string> = {
    ts: "typescript", tsx: "typescript", js: "javascript", jsx: "javascript",
    go: "go", sql: "sql", json: "json", css: "css", html: "html", md: "markdown",
    py: "python", sh: "shell", yml: "yaml", yaml: "yaml",
  };
  return map[ext] ?? "plaintext";
}

export function DiffEditorTab({
  repoPath, path, onSaved,
}: { repoPath: string; path: string; onSaved: () => void }) {
  const [original, setOriginal] = useState<string | null>(null);
  const [modified, setModified] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const editorRef = useRef<ModifiedGetter | null>(null);

  useEffect(() => { setupMonaco(); }, []);

  useEffect(() => {
    let active = true;
    void Promise.all([ReadRepoFileHead(repoPath, path), ReadRepoFile(repoPath, path)])
      .then(([head, work]) => {
        if (!active) return;
        setOriginal(head);
        setModified(work);
      })
      .catch((e) => { if (active) setError(e instanceof Error ? e.message : String(e)); });
    return () => { active = false; };
  }, [repoPath, path]);

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

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "4px 8px", borderBottom: "1px solid var(--border)" }}>
        <span style={{ fontSize: 11 }}>{path}</span>
        <button className="btn-primary" style={{ fontSize: 11, padding: "2px 10px", marginLeft: "auto" }}
          disabled={saving} onClick={() => void handleSave()}>
          {saving ? "Saving…" : "Save"}
        </button>
      </div>
      {error && <div style={{ fontSize: 11, color: "#dc2626", padding: "2px 8px" }}>{error}</div>}
      <div style={{ flex: 1, minHeight: 0 }}>
        {original !== null && modified !== null && (
          <DiffEditor
            original={original}
            modified={modified}
            language={languageFor(path)}
            options={{ readOnly: false, originalEditable: false, renderSideBySide: true, fontSize: 12 }}
            onMount={(editor) => { editorRef.current = editor as unknown as ModifiedGetter; }}
          />
        )}
      </div>
    </div>
  );
}
