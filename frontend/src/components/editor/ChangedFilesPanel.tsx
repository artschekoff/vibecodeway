import { useCallback, useEffect, useRef, useState } from "react";
import { GetContextChanges, StageFile, UnstageFile, DiscardFile } from "../../../wailsjs/go/main/App";
import { main } from "../../../wailsjs/go/models";

type Props = {
  contextId: string;
  onOpenFile: (repoPath: string, repoName: string, path: string, status: string) => void;
};

const POLL_MS = 2000;

const rowStyle: React.CSSProperties = {
  display: "flex", alignItems: "center", gap: 6, width: "100%",
  fontSize: 11, padding: "2px 4px", color: "var(--text)",
};
const nameBtnStyle: React.CSSProperties = {
  flex: 1, textAlign: "left", background: "none", border: "none", cursor: "pointer",
  color: "var(--text)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap",
};
const actionBtnStyle: React.CSSProperties = {
  background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)", padding: "0 2px",
};

export function ChangedFilesPanel({ contextId, onOpenFile }: Props) {
  const [repos, setRepos] = useState<main.RepoChange[]>([]);
  const mountedRef = useRef(true);

  const load = useCallback(async () => {
    try {
      const r = await GetContextChanges(contextId);
      if (mountedRef.current) setRepos(r ?? []);
    } catch {
      /* transient; keep last known */
    }
  }, [contextId]);

  useEffect(() => {
    let active = true;
    const tick = async () => { if (active) await load(); };
    void tick();
    const t = setInterval(tick, POLL_MS);
    return () => { active = false; mountedRef.current = false; clearInterval(t); };
  }, [load]);

  const act = async (fn: (repoPath: string, path: string) => Promise<void>, repoPath: string, path: string) => {
    try {
      await fn(repoPath, path);
    } finally {
      await load(); // reflect the change immediately, don't wait for the poll
    }
  };

  const onDiscard = (repoPath: string, path: string) => {
    if (window.confirm(`Discard changes to ${path}? This cannot be undone.`)) {
      void act(DiscardFile, repoPath, path);
    }
  };

  const statusLetter = (s: string) => (s ? s.charAt(0).toUpperCase() : "");

  if (repos.length === 0) {
    return <div style={{ fontSize: 12, color: "var(--text-muted)", padding: 12 }}>No changes</div>;
  }

  return (
    <div style={{ overflowY: "auto", padding: "8px 10px" }}>
      {repos.map((repo) => {
        const files = repo.files ?? [];
        const staged = files.filter((f) => f.indexStatus);
        const unstaged = files.filter((f) => f.workStatus);
        // Committed changes vs the base branch, with no pending working-tree edit
        // — read-only (opening shows the diff vs base); nothing to stage/discard.
        const branch = files.filter((f) => f.commitStatus && !f.indexStatus && !f.workStatus);
        return (
          <div key={repo.repoName} style={{ marginBottom: 12 }}>
            <div style={{ fontSize: 12, fontWeight: 600, marginBottom: 4 }}>{repo.repoName}</div>

            {staged.length > 0 && (
              <div style={{ marginBottom: 6 }}>
                <div style={{ fontSize: 10, textTransform: "uppercase", color: "var(--text-muted)", margin: "2px 0" }}>Staged Changes</div>
                {staged.map((f) => (
                  <div key={`s:${f.path}`} style={rowStyle}>
                    <span style={{ width: 14, color: "var(--text-muted)" }}>{statusLetter(f.indexStatus)}</span>
                    <button style={nameBtnStyle} onClick={() => onOpenFile(repo.repoPath, repo.repoName, f.path, f.indexStatus)}>{f.path}</button>
                    <button aria-label={`unstage ${f.path}`} title="Unstage" style={actionBtnStyle} onClick={() => void act(UnstageFile, repo.repoPath, f.path)}>−</button>
                  </div>
                ))}
              </div>
            )}

            {unstaged.length > 0 && (
              <div>
                <div style={{ fontSize: 10, textTransform: "uppercase", color: "var(--text-muted)", margin: "2px 0" }}>Changes</div>
                {unstaged.map((f) => (
                  <div key={`w:${f.path}`} style={rowStyle}>
                    <span style={{ width: 14, color: "var(--text-muted)" }}>{statusLetter(f.workStatus)}</span>
                    <button style={nameBtnStyle} onClick={() => onOpenFile(repo.repoPath, repo.repoName, f.path, f.workStatus)}>{f.path}</button>
                    <button aria-label={`discard ${f.path}`} title="Discard" style={actionBtnStyle} onClick={() => onDiscard(repo.repoPath, f.path)}>↺</button>
                    <button aria-label={`stage ${f.path}`} title="Stage" style={actionBtnStyle} onClick={() => void act(StageFile, repo.repoPath, f.path)}>+</button>
                  </div>
                ))}
              </div>
            )}

            {branch.length > 0 && (
              <div>
                <div style={{ fontSize: 10, textTransform: "uppercase", color: "var(--text-muted)", margin: "2px 0" }}>Branch Changes</div>
                {branch.map((f) => (
                  <div key={`b:${f.path}`} style={rowStyle}>
                    <span style={{ width: 14, color: "var(--text-muted)" }}>{statusLetter(f.commitStatus)}</span>
                    <button style={nameBtnStyle} onClick={() => onOpenFile(repo.repoPath, repo.repoName, f.path, f.commitStatus)}>{f.path}</button>
                  </div>
                ))}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
