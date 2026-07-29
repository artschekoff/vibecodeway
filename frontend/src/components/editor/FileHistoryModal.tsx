import { useEffect, useState } from "react";
import { DiffEditor } from "@monaco-editor/react";
import { FileHistory, FileDiffAtCommit } from "../../../wailsjs/go/main/App";
import type { main } from "../../../wailsjs/go/models";
import { useEditorSettings, toMonacoOptions } from "../../contexts/EditorSettingsContext";
import { languageFor } from "./DiffEditorTab";

const overlay: React.CSSProperties = {
  position: "fixed", inset: 0, zIndex: 100, background: "rgba(0,0,0,0.5)",
  display: "flex", alignItems: "center", justifyContent: "center",
};
const modal: React.CSSProperties = {
  width: "min(1100px, 92vw)", height: "min(760px, 88vh)", background: "var(--bg)",
  border: "1px solid var(--border)", borderRadius: 8, display: "flex", flexDirection: "column",
  overflow: "hidden", boxShadow: "0 10px 40px rgba(0,0,0,0.4)",
};

/** Git history for a single file: commit list + the selected commit's diff. */
export function FileHistoryModal({ repoPath, path, onClose }: { repoPath: string; path: string; onClose: () => void }) {
  const [commits, setCommits] = useState<main.FileCommit[]>([]);
  const [selected, setSelected] = useState<string>("");
  const [diff, setDiff] = useState<main.FileDiff | null>(null);
  const { settings } = useEditorSettings();
  const { theme, options } = toMonacoOptions(settings);

  useEffect(() => {
    let active = true;
    void FileHistory(repoPath, path).then((rows) => {
      if (!active) return;
      const list = rows ?? [];
      setCommits(list);
      if (list.length) setSelected(list[0].hash);
    }).catch(() => { if (active) setCommits([]); });
    return () => { active = false; };
  }, [repoPath, path]);

  useEffect(() => {
    if (!selected) return;
    let active = true;
    setDiff(null);
    void FileDiffAtCommit(repoPath, path, selected)
      .then((d) => { if (active) setDiff(d); })
      .catch(() => { if (active) setDiff(null); });
    return () => { active = false; };
  }, [repoPath, path, selected]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div style={overlay} onClick={onClose}>
      <div style={modal} onClick={(e) => e.stopPropagation()}>
        <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "8px 12px", borderBottom: "1px solid var(--border)" }}>
          <span style={{ fontSize: 12, fontWeight: 600 }}>History: {path}</span>
          <button aria-label="close history" onClick={onClose} style={{ marginLeft: "auto", background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)", fontSize: 16, lineHeight: 1 }}>×</button>
        </div>
        <div style={{ display: "flex", flex: 1, minHeight: 0 }}>
          <ul style={{ width: 300, margin: 0, padding: 0, listStyle: "none", overflowY: "auto", borderRight: "1px solid var(--border)" }}>
            {commits.length === 0 && <li style={{ fontSize: 11, color: "var(--text-muted)", padding: 12 }}>No history for this file.</li>}
            {commits.map((c) => (
              <li key={c.hash}>
                <button
                  onClick={() => setSelected(c.hash)}
                  style={{
                    display: "block", width: "100%", textAlign: "left", cursor: "pointer",
                    background: c.hash === selected ? "var(--bg-hover, rgba(127,127,127,0.15))" : "none",
                    border: "none", borderBottom: "1px solid var(--border)", padding: "6px 10px", color: "var(--text)",
                  }}
                >
                  <div style={{ fontSize: 11, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{c.subject}</div>
                  <div style={{ fontSize: 10, color: "var(--text-muted)", marginTop: 2 }}>
                    {c.hash.slice(0, 8)} · {c.author} · {c.date.slice(0, 10)}
                  </div>
                </button>
              </li>
            ))}
          </ul>
          <div style={{ flex: 1, minWidth: 0 }}>
            {diff && (
              <DiffEditor
                original={diff.before}
                modified={diff.after}
                language={languageFor(path)}
                theme={theme}
                options={{ ...options, readOnly: true, originalEditable: false }}
              />
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
