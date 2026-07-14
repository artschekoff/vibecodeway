// reviewStatusMeta maps an internal review_status value to its badge label, CSS class,
// and whether it represents a dismissed prior approval that now needs re-approval.
// Keep the "re_review_required" string in sync with reviewStatusReReviewRequired in
// internal/vcs/github/runner.go.
export function reviewStatusMeta(reviewStatus: string): {
  label: string | null;
  className: string;
  needsReReview: boolean;
} {
  switch (reviewStatus) {
    case "changes_requested":
      return { label: "Changes Requested", className: " pr-status--changes-requested", needsReReview: false };
    case "approved":
      return { label: "Approved", className: " pr-status--approved", needsReReview: false };
    case "re_review_required":
      return { label: "Re-approval Needed", className: " pr-status--re-review", needsReReview: true };
    case "review_required":
      return { label: "Review Required", className: "", needsReReview: false };
    default:
      return { label: null, className: "", needsReReview: false };
  }
}
