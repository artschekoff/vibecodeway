export function PRStatusBadge({ status, reviewStatus, mergeable, ciStatus, isDraft }: { status: string; reviewStatus: string; mergeable?: string; ciStatus?: string; isDraft?: boolean }) {
  const isMerged = status === "merged";
  const isClosed = status === "closed";

  const reviewLabel = !isMerged && !isClosed
    ? (reviewStatus === "changes_requested" ? "Changes Requested"
      : reviewStatus === "approved" ? "Approved"
      : reviewStatus === "review_required" ? "Review Required"
      : reviewStatus || null)
    : null;

  const isReadyToMerge = !isMerged && !isClosed
    && ciStatus === "SUCCESS" && reviewStatus === "approved" && mergeable === "MERGEABLE";

  return (
    <div className="pr-badges">
      {isDraft && <span className="pr-status-badge pr-status-badge--draft">Draft</span>}
      <span className={`pr-status-badge pr-status-badge--${status}`}>{status}</span>
      {reviewLabel && (
        <span className={`pr-status-badge pr-review-badge${reviewStatus === "changes_requested" ? " pr-status--changes-requested" : reviewStatus === "approved" ? " pr-status--approved" : ""}`}>
          {reviewLabel}
        </span>
      )}
      {!isMerged && !isClosed && mergeable === "CONFLICTING" && (
        <span className="pr-status-badge pr-status-badge--conflict" title="This PR has merge conflicts">
          Conflicts
        </span>
      )}
      {!isMerged && !isClosed && ciStatus === "PENDING" && (
        <span className="pr-status-badge pr-status-badge--ci-pending" title="CI checks running">
          CI running
        </span>
      )}
      {!isMerged && !isClosed && ciStatus === "FAILURE" && (
        <span className="pr-status-badge pr-status-badge--ci-failure" title="CI checks failed">
          CI failed
        </span>
      )}
      {isReadyToMerge && (
        <span className="pr-status-badge pr-status-badge--ready" title="All checks passed and approved — ready to merge">
          Ready to merge
        </span>
      )}
    </div>
  );
}
