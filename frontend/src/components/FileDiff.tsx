type Props = { path: string; status: string; diff: string };

export default function FileDiff({ path, status, diff }: Props) {
  const lines = diff ? diff.split("\n") : [];
  return (
    <div className="file-diff">
      <div className="file-diff-header">
        <span className="file-diff-status">{status}</span>
        <span className="file-diff-path">{path}</span>
      </div>
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
    </div>
  );
}
