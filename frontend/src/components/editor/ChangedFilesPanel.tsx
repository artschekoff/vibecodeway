import { useCallback, useEffect, useRef, useState } from "react";
import { Copy, Terminal, ChevronsDownUp, ChevronsUpDown, RefreshCw } from "lucide-react";
import { GetContextChanges, StageFile, UnstageFile, DiscardFile, ConfirmAction, ResolveConflict } from "../../../wailsjs/go/main/App";
import { main } from "../../../wailsjs/go/models";

type Props = {
  contextId: string;
  onOpenFile: (repoPath: string, repoName: string, path: string, status: string, baseMode: "branch" | "head") => void;
  // Open a new in-app terminal at the repo's local path.
  onOpenTerminal?: (repoPath: string) => void;
  // Re-sync the context from GitHub (icon shown in the repo header).
  onRefresh?: () => void;
  refreshing?: boolean;
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
  fontSize: 11, fontFamily: "inherit", // buttons don't inherit font — keep names at row size
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

// Sections are collapsed by default; this stores which ones the user opened.
const EXPAND_KEY = "vibecodeway:changedfiles:expanded";

function loadExpanded(): Set<string> {
  try {
    const raw = localStorage.getItem(EXPAND_KEY);
    return new Set(raw ? (JSON.parse(raw) as string[]) : []);
  } catch {
    return new Set();
  }
}

const toggleAllBtnStyle: React.CSSProperties = {
  display: "flex", alignItems: "center", background: "none", border: "1px solid var(--border)",
  borderRadius: 4, cursor: "pointer", color: "var(--text-muted)", padding: "3px 5px", flexShrink: 0,
};

export function ChangedFilesPanel({ contextId, onOpenFile, onOpenTerminal, onRefresh, refreshing, activeRepoPath, refreshSignal }: Props) {
  const [repos, setRepos] = useState<main.RepoChange[]>([]);
  const [expanded, setExpanded] = useState<Set<string>>(() => loadExpanded());
  const [filter, setFilter] = useState("");
  const mountedRef = useRef(true);

  const persistExpanded = (next: Set<string>) => {
    try { localStorage.setItem(EXPAND_KEY, JSON.stringify([...next])); } catch { /* ignore */ }
  };
  const toggleSection = (key: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      persistExpanded(next);
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

  const onDiscard = async (repoPath: string, path: string) => {
    // Native dialog — window.confirm() is a no-op in the Wails webview.
    if (await ConfirmAction("Discard changes", `Discard changes to ${path}? This cannot be undone.`)) {
      void act(DiscardFile, repoPath, path);
    }
  };

  const statusLetter = (s: string) => (s ? s.charAt(0).toUpperCase() : "");
  // Git status letter → colour: added/untracked green, deleted red, else (M/R/C) orange.
  const statusColor = (s: string) => {
    switch (statusLetter(s)) {
      case "A": case "U": case "?": return "#3fb950"; // green
      case "D": return "#f85149"; // red
      default: return "#d29922"; // orange (modified/renamed/etc.)
    }
  };

  const shown = activeRepoPath ? repos.filter((r) => r.repoPath === activeRepoPath) : repos;

  if (shown.length === 0) {
    return <div style={{ fontSize: 12, color: "var(--text-muted)", padding: 12 }}>No changes</div>;
  }

  const q = filter.trim().toLowerCase();
  const matchFile = (p: string) => !q || p.toLowerCase().includes(q);
  // A section is open when explicitly expanded, or while a filter is active
  // (so matches are visible even though sections default to collapsed).
  const sectionOpen = (key: string) => q !== "" || expanded.has(key);

  // Keys of every section that currently has (matching) content — for expand-all.
  const allSectionKeys: string[] = [];
  for (const repo of shown) {
    const files = (repo.files ?? []).filter((f) => matchFile(f.path));
    if (files.some((f) => f.commitStatus)) allSectionKeys.push(`${repo.repoName}:branch`);
    if (files.some((f) => f.indexStatus)) allSectionKeys.push(`${repo.repoName}:staged`);
    if (files.some((f) => f.workStatus)) allSectionKeys.push(`${repo.repoName}:local`);
  }
  const toggleAll = () => {
    const next = expanded.size ? new Set<string>() : new Set(allSectionKeys);
    setExpanded(next);
    persistExpanded(next);
  };

  return (
    <div style={{ overflowY: "auto", padding: "8px 10px" }}>
      <div style={{ display: "flex", gap: 6, marginBottom: 8 }}>
        <input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="Filter files…"
          aria-label="filter changed files"
          style={{ flex: 1, minWidth: 0, boxSizing: "border-box", fontSize: 11, padding: "3px 6px", background: "var(--surface2)", border: "1px solid var(--border)", borderRadius: 4, color: "var(--text)" }}
        />
        <button onClick={toggleAll} style={toggleAllBtnStyle}
          aria-label={expanded.size ? "collapse all" : "expand all"}
          title={expanded.size ? "Collapse all" : "Expand all"}>
          {expanded.size ? <ChevronsDownUp size={13} /> : <ChevronsUpDown size={13} />}
        </button>
      </div>
      {shown.map((repo) => {
        const files = (repo.files ?? []).filter((f) => matchFile(f.path));
        // Branch (PR) changes vs the base branch — the full committed set, shown
        // on top like GitHub. Read-only: nothing to stage/discard.
        const branch = files.filter((f) => f.commitStatus);
        // Local uncommitted changes (independent of the branch list — a file in
        // the PR that's also edited locally appears in both).
        // Files git left unmerged. They carry no index/work status precisely so
        // they land here alone instead of being double-listed below.
        const conflicts = files.filter((f) => f.conflicted);
        const staged = files.filter((f) => f.indexStatus);
        const unstaged = files.filter((f) => f.workStatus);
        // Keep the repo header (and its refresh/copy/terminal actions) visible even
        // with no changes — otherwise the refresh button would vanish exactly when
        // you'd want to re-check for changes.
        const bKey = `${repo.repoName}:branch`;
        const sKey = `${repo.repoName}:staged`;
        const lKey = `${repo.repoName}:local`;
        const cKey = `${repo.repoName}:conflicts`;
        return (
          <div key={repo.repoName} style={{ marginBottom: 12 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 4 }}>
              <span style={{ flex: 1, fontSize: 12, fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={repo.repoPath}>{repo.repoName}</span>
              <button aria-label={`copy path for ${repo.repoName}`} title="Copy repo path" style={actionBtnStyle} onClick={() => void navigator.clipboard?.writeText(repo.repoPath)}><Copy size={12} /></button>
              <button aria-label={`open terminal in ${repo.repoName}`} title="Open terminal here" style={actionBtnStyle} onClick={() => onOpenTerminal?.(repo.repoPath)}><Terminal size={12} /></button>
              {onRefresh && (
                <button aria-label="refresh from github" title="Refresh from GitHub" style={actionBtnStyle} disabled={refreshing} onClick={onRefresh}>
                  <RefreshCw size={12} className={refreshing ? "spinning" : undefined} />
                </button>
              )}
            </div>

            {conflicts.length > 0 && (
              <div style={{ marginBottom: 6 }}>
                <button style={headerBtnStyle} onClick={() => toggleSection(cKey)}>
                  <span style={chevronStyle}>{sectionOpen(cKey) ? "▾" : "▸"}</span>Merge Conflicts
                </button>
                {sectionOpen(cKey) && conflicts.map((f) => (
                  <div key={`c:${f.path}`} style={rowStyle}>
                    <span style={{ width: 14, color: "#f85149" }} title="Unmerged">!</span>
                    {/* Opens the working-tree file with its conflict markers, so the
                        finer-grained resolution is editing it and marking it resolved. */}
                    <button style={nameBtnStyle} onClick={() => onOpenFile(repo.repoPath, repo.repoName, f.path, "conflict", "head")}>{f.path}</button>
                    <button aria-label={`keep ours for ${f.path}`} title="Keep ours (this branch)" style={actionBtnStyle}
                      onClick={() => void act((rp, p) => ResolveConflict(rp, p, "ours"), repo.repoPath, f.path)}>ours</button>
                    <button aria-label={`keep theirs for ${f.path}`} title="Keep theirs (branch being merged in)" style={actionBtnStyle}
                      onClick={() => void act((rp, p) => ResolveConflict(rp, p, "theirs"), repo.repoPath, f.path)}>theirs</button>
                    <button aria-label={`mark ${f.path} resolved`} title="Mark resolved (stage it)" style={actionBtnStyle}
                      onClick={() => void act(StageFile, repo.repoPath, f.path)}>✓</button>
                  </div>
                ))}
              </div>
            )}

            {branch.length > 0 && (
              <div style={{ marginBottom: 6 }}>
                <button style={headerBtnStyle} onClick={() => toggleSection(bKey)}>
                  <span style={chevronStyle}>{sectionOpen(bKey) ? "▾" : "▸"}</span>Branch Changes
                </button>
                {sectionOpen(bKey) && branch.map((f) => (
                  <div key={`b:${f.path}`} style={rowStyle}>
                    <span style={{ width: 14, color: statusColor(f.commitStatus) }}>{statusLetter(f.commitStatus)}</span>
                    <button style={nameBtnStyle} onClick={() => onOpenFile(repo.repoPath, repo.repoName, f.path, f.commitStatus, "branch")}>{f.path}</button>
                  </div>
                ))}
              </div>
            )}

            {staged.length > 0 && (
              <div style={{ marginBottom: 6 }}>
                <button style={headerBtnStyle} onClick={() => toggleSection(sKey)}>
                  <span style={chevronStyle}>{sectionOpen(sKey) ? "▾" : "▸"}</span>Staged Changes
                </button>
                {sectionOpen(sKey) && staged.map((f) => (
                  <div key={`s:${f.path}`} style={rowStyle}>
                    <span style={{ width: 14, color: statusColor(f.indexStatus) }}>{statusLetter(f.indexStatus)}</span>
                    <button style={nameBtnStyle} onClick={() => onOpenFile(repo.repoPath, repo.repoName, f.path, f.indexStatus, "head")}>{f.path}</button>
                    <button aria-label={`unstage ${f.path}`} title="Unstage" style={actionBtnStyle} onClick={() => void act(UnstageFile, repo.repoPath, f.path)}>−</button>
                  </div>
                ))}
              </div>
            )}

            {unstaged.length > 0 && (
              <div>
                <button style={headerBtnStyle} onClick={() => toggleSection(lKey)}>
                  <span style={chevronStyle}>{sectionOpen(lKey) ? "▾" : "▸"}</span>Local Changes
                </button>
                {sectionOpen(lKey) && unstaged.map((f) => (
                  <div key={`w:${f.path}`} style={rowStyle}>
                    <span style={{ width: 14, color: statusColor(f.workStatus) }}>{statusLetter(f.workStatus)}</span>
                    <button style={nameBtnStyle} onClick={() => onOpenFile(repo.repoPath, repo.repoName, f.path, f.workStatus, "head")}>{f.path}</button>
                    <button aria-label={`discard ${f.path}`} title="Discard" style={actionBtnStyle} onClick={() => void onDiscard(repo.repoPath, f.path)}>↺</button>
                    <button aria-label={`stage ${f.path}`} title="Stage" style={actionBtnStyle} onClick={() => void act(StageFile, repo.repoPath, f.path)}>+</button>
                  </div>
                ))}
              </div>
            )}

            {files.length === 0 && (
              <div style={{ fontSize: 11, color: "var(--text-muted)", padding: "2px 4px" }}>
                {q ? "No matching files" : "No changes"}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
