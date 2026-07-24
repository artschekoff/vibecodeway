import { useEffect, useState } from "react";
import { GetContextChanges } from "../../../wailsjs/go/main/App";
import { main } from "../../../wailsjs/go/models";

type Props = {
  contextId: string;
  onOpenFile: (repoPath: string, repoName: string, path: string, status: string) => void;
};

const POLL_MS = 2000;

export function ChangedFilesPanel({ contextId, onOpenFile }: Props) {
  const [repos, setRepos] = useState<main.RepoChange[]>([]);

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const r = await GetContextChanges(contextId);
        if (active) setRepos(r ?? []);
      } catch {
        /* transient; keep last known */
      }
    };
    void load();
    const t = setInterval(load, POLL_MS);
    return () => {
      active = false;
      clearInterval(t);
    };
  }, [contextId]);

  if (repos.length === 0) {
    return <div style={{ fontSize: 12, color: "var(--text-muted)", padding: 12 }}>No changes</div>;
  }

  return (
    <div style={{ overflowY: "auto", padding: "8px 10px" }}>
      {repos.map((repo) => (
        <div key={repo.repoName} style={{ marginBottom: 12 }}>
          <div style={{ fontSize: 12, fontWeight: 600, marginBottom: 4 }}>{repo.repoName}</div>
          {(repo.files ?? []).map((f) => (
            <button
              key={f.path}
              onClick={() => onOpenFile(repo.repoPath, repo.repoName, f.path, f.status)}
              style={{
                display: "flex", gap: 8, width: "100%", textAlign: "left",
                background: "none", border: "none", cursor: "pointer",
                fontSize: 11, padding: "2px 4px", color: "var(--text)",
              }}
            >
              <span style={{ width: 14, color: "var(--text-muted)" }}>{f.status.charAt(0).toUpperCase()}</span>
              <span>{f.path}</span>
            </button>
          ))}
        </div>
      ))}
    </div>
  );
}
