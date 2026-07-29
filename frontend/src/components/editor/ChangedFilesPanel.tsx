import { useCallback, useEffect, useRef, useState } from "react";
import { GetContextChanges, StageFile, UnstageFile, DiscardFile } from "../../../wailsjs/go/main/App";
import { main } from "../../../wailsjs/go/models";

type Props = {
  contextId: string;
  onOpenFile: (repoPath: string, repoName: string, path: string, status: string) => void;
  // When set, show only the repo whose repoPath matches (follows the editor's
  // repo selector). Unset => show all repos.
  activeRepoPath?: string;
  // Bumped by the sidebar Refresh button to force an immediate re-fetch (the panel
  // also polls on its own interval).
  refreshSignal?: number;
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
const headerBtnStyle: React.CSSProperties = {
  display: "flex", alignItems: "center", gap: 4, width: "100%", background: "none", border: "none",
  cursor: "pointer", padding: 0, margin: "2px 0", textAlign: "left",
  fontSize: 10, textTransform: "uppercase", color: "var(--text-muted)",
};
const chevronStyle: React.CSSProperties = { width: 10, display: "inline-block", fontSize: 9 };

const COLLAPSE_KEY = "context-consolidator:changedfiles:collapsed";

function loadCollapsed(): Set<string> {
  try {
    const raw = localStorage.getItem(COLLAPSE_KEY);
    return new Set(raw ? (JSON.parse(raw) as string[]) : []);
  } catch {
    return new Set();
  }
}

export function ChangedFilesPanel({ contextId, onOpenFile, activeRepoPath, refreshSignal }: Props) {
  const [repos, setRepos] = useState<main.RepoChange[]>([]);
  const [collapsed, setCollapsed] = useState<Set<string>>(() => loadCollapsed());
  const mountedRef = useRef(true);

  const toggleSection = (key: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      try {
        localStorage.setItem(COLLAPSE_KEY, JSON.stringify([...next]));
      } catch {
        /* ignore persistence failures */
      }
      return next;
    });
  };

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
    mountedRef.current = true; // re-arm on every (re)run — deps now include refreshSignal
    const tick = async () => { if (active) await load(); };
    void tick();
    const t = setInterval(tick, POLL_MS);
    return () => { active = false; mountedRef.current = false; clearInterval(t); };
  }, [load, refreshSignal]);

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

  const shown = activeRepoPath ? repos.filter((r) => r.repoPath === activeRepoPath) : repos;

  if (shown.length === 0) {
    return <div style={{ fontSize: 12, color: "var(--text-muted)", padding: 12 }}>No changes</div>;
  }

  return (
    <div style={{ overflowY: "auto", padding: "8px 10px" }}>
      {shown.map((repo) => {
        const files = repo.files ?? [];
        // Branch (PR) changes vs the base branch — the full committed set, shown
        // on top like GitHub. Read-only: nothing to stage/discard.
        const branch = files.filter((f) => f.commitStatus);
        // Local uncommitted changes (independent of the branch list — a file in
        // the PR that's also edited locally appears in both).
        const staged = files.filter((f) => f.indexStatus);
        const unstaged = files.filter((f) => f.workStatus);
        const bKey = `${repo.repoName}:branch`;
        const sKey = `${repo.repoName}:staged`;
        const lKey = `${repo.repoName}:local`;
        return (
          <div key={repo.repoName} style={{ marginBottom: 12 }}>
            <div style={{ fontSize: 12, fontWeight: 600, marginBottom: 4 }}>{repo.repoName}</div>

            {branch.length > 0 && (
              <div style={{ marginBottom: 6 }}>
                <button style={headerBtnStyle} onClick={() => toggleSection(bKey)}>
                  <span style={chevronStyle}>{collapsed.has(bKey) ? "▸" : "▾"}</span>Branch Changes
                </button>
                {!collapsed.has(bKey) && branch.map((f) => (
                  <div key={`b:${f.path}`} style={rowStyle}>
                    <span style={{ width: 14, color: "var(--text-muted)" }}>{statusLetter(f.commitStatus)}</span>
                    <button style={nameBtnStyle} onClick={() => onOpenFile(repo.repoPath, repo.repoName, f.path, f.commitStatus)}>{f.path}</button>
                  </div>
                ))}
              </div>
            )}

            {staged.length > 0 && (
              <div style={{ marginBottom: 6 }}>
                <button style={headerBtnStyle} onClick={() => toggleSection(sKey)}>
                  <span style={chevronStyle}>{collapsed.has(sKey) ? "▸" : "▾"}</span>Staged Changes
                </button>
                {!collapsed.has(sKey) && staged.map((f) => (
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
                <button style={headerBtnStyle} onClick={() => toggleSection(lKey)}>
                  <span style={chevronStyle}>{collapsed.has(lKey) ? "▸" : "▾"}</span>Local Changes
                </button>
                {!collapsed.has(lKey) && unstaged.map((f) => (
                  <div key={`w:${f.path}`} style={rowStyle}>
                    <span style={{ width: 14, color: "var(--text-muted)" }}>{statusLetter(f.workStatus)}</span>
                    <button style={nameBtnStyle} onClick={() => onOpenFile(repo.repoPath, repo.repoName, f.path, f.workStatus)}>{f.path}</button>
                    <button aria-label={`discard ${f.path}`} title="Discard" style={actionBtnStyle} onClick={() => onDiscard(repo.repoPath, f.path)}>↺</button>
                    <button aria-label={`stage ${f.path}`} title="Stage" style={actionBtnStyle} onClick={() => void act(StageFile, repo.repoPath, f.path)}>+</button>
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
