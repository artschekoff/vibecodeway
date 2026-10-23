import { useEffect, useState } from "react";
import { SolveWithAI, ListVCSBranches } from "../../wailsjs/go/main/App";
import { ProseEditor } from "./ProseEditor";

interface Props {
  contextId: string;
  onClose: () => void;
  /** Called with the worktree the agent works in, so the editor opens right there. */
  onOpenEditor?: (worktreePath: string) => void;
}

type Target = { repoId: string; branch: string; label: string };
const targetKey = (t: Target) => `${t.repoId}\u0000${t.branch}`;

/**
 * Solve with AI: pick the branch (repo) to work in — only that one gets a
 * worktree and the on-checkout script — then build the agent task and open the
 * editor there. A single linked branch is preselected.
 */
export function SolveWithAIModal({ contextId, onClose, onOpenEditor }: Props) {
  const [task, setTask] = useState("implement");
  const [targets, setTargets] = useState<Target[]>([]);
  const [targetsLoaded, setTargetsLoaded] = useState(false);
  const [picked, setPicked] = useState("");
  const [loading, setLoading] = useState(false);
  const [done, setDone] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    ListVCSBranches(contextId)
      .then((rows) => {
        if (!alive) return;
        const seen = new Set<string>();
        const list: Target[] = [];
        for (const b of rows ?? []) {
          if (!b.branch) continue;
          const t = { repoId: b.repoId, branch: b.branch, label: `${b.repo ? `${b.repo.owner}/${b.repo.name}` : b.repoId} · ${b.branch}` };
          if (seen.has(targetKey(t))) continue;
          seen.add(targetKey(t));
          list.push(t);
        }
        setTargets(list);
        if (list.length === 1) setPicked(targetKey(list[0]));
      })
      .catch(() => { /* no branches: the backend error on Solve says so */ })
      .finally(() => { if (alive) setTargetsLoaded(true); });
    return () => { alive = false; };
  }, [contextId]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  const target = targets.find((t) => targetKey(t) === picked);

  const handleConfirm = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await SolveWithAI(contextId, task, target?.repoId ?? "", target?.branch ?? "");
      await navigator.clipboard.writeText(res.instruction).catch(() => {});
      setDone(true);
      onOpenEditor?.(res.worktreePath);
    } catch (err) {
      // Wails rejects with a plain string, not an Error — coerce whatever we got.
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg || "Unknown error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="solve-with-ai-modal-title"
      onClick={onClose}
      style={{
        position: "fixed",
        inset: 0,
        background: "rgba(0,0,0,0.4)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 50,
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          background: "var(--surface)",
          border: "1px solid var(--border)",
          borderRadius: 8,
          padding: 20,
          maxWidth: 640,
          width: "90%",
          maxHeight: "80vh",
          overflowY: "auto",
        }}
      >
        <div id="solve-with-ai-modal-title" style={{ fontSize: 12, fontWeight: 600, marginBottom: 10 }}>
          Solve with AI
        </div>
        <ProseEditor
          value={task}
          onChange={(e) => setTask(e.target.value)}
          rows={4}
          style={{
            width: "100%",
            lineHeight: 1.6,
            marginBottom: 16,
            color: "var(--text)",
            background: "var(--surface2)",
            border: "1px solid var(--border)",
            borderRadius: 6,
            padding: "8px 10px",
            resize: "vertical"
          }}
        />
        {targets.length > 1 && (
          <label style={{ display: "flex", flexDirection: "column", gap: 4, fontSize: 11, marginBottom: 12 }}>
            Work in
            <select aria-label="Repository and branch" value={picked} onChange={(e) => setPicked(e.target.value)}>
              <option value="">Choose a repository…</option>
              {targets.map((t) => <option key={targetKey(t)} value={targetKey(t)}>{t.label}</option>)}
            </select>
          </label>
        )}
        {targets.length === 1 && (
          <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 12 }}>Work in {targets[0].label}</div>
        )}
        {targetsLoaded && targets.length === 0 && (
          <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 12 }}>No branch is linked to this context yet.</div>
        )}
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
          <button
            className="btn-primary"
            style={{ fontSize: 11, padding: "4px 12px" }}
            disabled={loading || !target}
            onClick={() => void handleConfirm()}
          >
            {loading ? "Solving…" : "Solve"}
          </button>
          <button
            aria-label="Close solve with AI"
            className="btn-secondary"
            style={{ fontSize: 11, padding: "4px 10px", marginLeft: "auto" }}
            onClick={onClose}
          >
            Close
          </button>
        </div>
        {error && (
          <div style={{ fontSize: 11, color: "#dc2626", marginTop: 8 }}>{error}</div>
        )}
        {done && (
          <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 8 }}>
            Task copied to clipboard — paste it into the terminal of the editor that just opened.
          </div>
        )}
      </div>
    </div>
  );
}
