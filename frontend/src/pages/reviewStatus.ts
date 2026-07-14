// reviewStatusMeta maps an internal review_status value to its badge label, CSS class,
// whether it needs re-approval (a dismissed prior approval), and whether it is a
// terminal PR lifecycle state (merged/closed). Keep the "re_review_required",
// "merged", and "closed" strings in sync with the Go backend (runner.go review status
// and vcs.PRData.State).
export function reviewStatusMeta(reviewStatus: string): {
  label: string | null;
  className: string;
  needsReReview: boolean;
  terminal: boolean;
} {
  switch (reviewStatus) {
    case "changes_requested":
      return { label: "Changes Requested", className: " pr-status--changes-requested", needsReReview: false, terminal: false };
    case "approved":
      return { label: "Approved", className: " pr-status--approved", needsReReview: false, terminal: false };
    case "re_review_required":
      return { label: "Re-approval Needed", className: " pr-status--re-review", needsReReview: true, terminal: false };
    case "review_required":
      return { label: "Review Required", className: "", needsReReview: false, terminal: false };
    case "merged":
      return { label: "Merged", className: " pr-status--merged", needsReReview: false, terminal: true };
    case "closed":
      return { label: "Closed", className: " pr-status--closed", needsReReview: false, terminal: true };
    default:
      return { label: null, className: "", needsReReview: false, terminal: false };
  }
}
