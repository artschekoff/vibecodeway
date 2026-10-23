import { useEffect, useState } from "react";
import { ChevronDown } from "lucide-react";
import { GetFileDiff } from "../../wailsjs/go/main/App";

type Props = {
  repoPath: string;
  path: string;
  status: string;
  // Which diff to compute lazily: "untracked" | "branch" | "head".
  mode: string;
  // Follow-the-leader signal from the "Expand/Collapse all" toggle; the header
  // click still flips each file independently afterwards.
  expandAll?: boolean;
};

// Diff text loads lazily on first expand (VS Code-style): the changes list
// only carries file names, so a repo with thousands of untracked files no
// longer ships megabytes of content nobody opened. +/− counts appear once
// the diff has loaded.
export default function FileDiff({ repoPath, path, status, mode, expandAll = false }: Props) {
  const [open, setOpen] = useState(expandAll);
  const [diff, setDiff] = useState<string | null>(null);
  useEffect(() => { setOpen(expandAll); }, [expandAll]);

  useEffect(() => {
    if (!open || diff !== null) return;
    let active = true;
    GetFileDiff(repoPath, path, mode)
      .then((d) => { if (active) setDiff(d ?? ""); })
      .catch(() => { if (active) setDiff(""); });
    return () => { active = false; };
  }, [open, diff, repoPath, path, mode]);

  const lines = diff ? diff.split("\n") : [];
  const added = lines.filter((l) => l.startsWith("+") && !l.startsWith("+++")).length;
  const removed = lines.filter((l) => l.startsWith("-") && !l.startsWith("---")).length;

  return (
    <div className="file-diff">
      <div
        className="file-diff-header"
        role="button"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        style={{ cursor: "pointer", userSelect: "none" }}
      >
        <span className="file-diff-status">{status}</span>
        <span className="file-diff-path">{path}</span>
        <span style={{ marginLeft: "auto", display: "inline-flex", alignItems: "center", gap: 8, fontSize: 11 }}>
          {diff !== null && (
            <>
              <span style={{ color: "var(--success, #3fb950)" }}>+{added}</span>
              <span style={{ color: "var(--danger, #e5534b)" }}>−{removed}</span>
            </>
          )}
          <ChevronDown size={13} style={{ transform: open ? "rotate(180deg)" : undefined, transition: "transform 0.15s", color: "var(--text-muted)" }} />
        </span>
      </div>
      {open && (
        diff === null ? (
          <pre className="file-diff-body"><div className="diff-line">Loading…</div></pre>
        ) : (
          <pre className="file-diff-body">
            {lines.map((ln, i) => {
              let cls = "diff-line";
              if (ln.startsWith("+") && !ln.startsWith("+++")) cls += " diff-line--added";
              else if (ln.startsWith("-") && !ln.startsWith("---")) cls += " diff-line--removed";
              else if (ln.startsWith("@@")) cls += " diff-line--hunk";
              else if (ln.startsWith("diff ") || ln.startsWith("index ") || ln.startsWith("+++") || ln.startsWith("---")) cls += " diff-line--meta";
              return <div key={i} className={cls}>{ln || " "}</div>;
            })}
          </pre>
        )
      )}
    </div>
  );
}
