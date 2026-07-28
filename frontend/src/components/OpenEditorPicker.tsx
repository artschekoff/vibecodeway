import { useEffect, useMemo, useState } from "react";
import { ListLocalBranches } from "../../wailsjs/go/main/App";
import type { main } from "../../wailsjs/go/models";
import type { LocalBranch } from "../types";

interface Props {
  choices: main.EditorRepoChoice[];
  error?: string | null;
  loading?: boolean;
  onConfirm: (pathId: string, branch: string) => void;
  onCancel: () => void;
}

/**
 * Two-step checkout picker shown when a context has no ready workspace: pick a
 * repo (and its local path when it has more than one), then a local branch
 * (defaulting to the repo's current branch). Confirming checks the branch out
 * into a worktree and opens the editor there.
 */
export function OpenEditorPicker({ choices, error, loading, onConfirm, onCancel }: Props) {
  const [repoId, setRepoId] = useState(choices[0]?.repoId ?? "");
  const repo = useMemo(() => choices.find((c) => c.repoId === repoId) ?? choices[0], [choices, repoId]);
  const paths = repo?.paths ?? [];
  const [pathId, setPathId] = useState(paths[0]?.id ?? "");
  const [branches, setBranches] = useState<LocalBranch[]>([]);
  const [branch, setBranch] = useState("");
  const [branchErr, setBranchErr] = useState<string | null>(null);

  // Reset the selected path whenever the repo changes.
  useEffect(() => {
    setPathId(repo?.paths?.[0]?.id ?? "");
  }, [repo]);

  // Load local branches for the chosen path; default to the current branch.
  useEffect(() => {
    if (!pathId) {
      setBranches([]);
      setBranch("");
      return;
    }
    let active = true;
    setBranchErr(null);
    void ListLocalBranches(pathId)
      .then((bs) => {
        if (!active) return;
        const list = (bs as LocalBranch[]) ?? [];
        setBranches(list);
        const current = list.find((b) => b.isCurrent);
        setBranch(current?.name ?? list[0]?.name ?? "");
      })
      .catch((e) => { if (active) setBranchErr(e instanceof Error ? e.message : String(e)); });
    return () => { active = false; };
  }, [pathId]);

  const canConfirm = !!pathId && !!branch && !loading;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Open editor — choose repo and branch"
      onClick={onCancel}
      style={{ position: "fixed", inset: 0, background: "rgba(0,0,0,0.4)", display: "flex", alignItems: "center", justifyContent: "center", zIndex: 50 }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{ background: "var(--surface)", border: "1px solid var(--border)", borderRadius: 8, padding: 20, maxWidth: 520, width: "90%" }}
      >
        <div style={{ fontSize: 12, fontWeight: 600, marginBottom: 12 }}>Open in editor — checkout a branch</div>

        <label style={{ fontSize: 11, color: "var(--text-muted)", display: "block", marginBottom: 4 }}>Repository</label>
        <select value={repoId} onChange={(e) => setRepoId(e.target.value)} style={{ width: "100%", fontSize: 12, marginBottom: 12 }}>
          {choices.map((c) => <option key={c.repoId} value={c.repoId}>{c.repoName}</option>)}
        </select>

        {paths.length > 1 && (
          <>
            <label style={{ fontSize: 11, color: "var(--text-muted)", display: "block", marginBottom: 4 }}>Local path</label>
            <select value={pathId} onChange={(e) => setPathId(e.target.value)} style={{ width: "100%", fontSize: 12, marginBottom: 12 }}>
              {paths.map((p) => <option key={p.id} value={p.id}>{p.path}</option>)}
            </select>
          </>
        )}

        <label style={{ fontSize: 11, color: "var(--text-muted)", display: "block", marginBottom: 4 }}>Branch</label>
        <select value={branch} onChange={(e) => setBranch(e.target.value)} style={{ width: "100%", fontSize: 12, marginBottom: 12 }} disabled={branches.length === 0}>
          {branches.map((b) => <option key={b.name} value={b.name}>{b.name}{b.isCurrent ? " (current)" : ""}</option>)}
        </select>

        {(error || branchErr) && (
          <div style={{ fontSize: 11, color: "#dc2626", marginBottom: 10 }}>{error || branchErr}</div>
        )}

        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <button className="btn-secondary" style={{ fontSize: 11, padding: "4px 12px" }} onClick={onCancel}>Cancel</button>
          <button className="btn-primary" style={{ fontSize: 11, padding: "4px 12px" }} disabled={!canConfirm} onClick={() => onConfirm(pathId, branch)}>
            {loading ? "Opening…" : "Checkout & Open"}
          </button>
        </div>
      </div>
    </div>
  );
}
