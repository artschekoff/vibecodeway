import type { ReviewRequest } from "../types";
import { reviewStatusMeta } from "./reviewStatus";

// ReviewStage is the single answer to "where does this PR stand for me right now?".
// It folds the PR-wide decision (reviewStatus), my own standing decision
// (myReviewState), the dropped-off-the-search heuristic (reviewedAt), and whether
// the PR is mine (kind) into one bucket that every Code Reviews surface groups by.
export type ReviewStage =
  | "awaiting_first"
  | "re_review"
  | "approved"
  | "reviewed_waiting"
  | "terminal"
  | "authored";

// REQUESTED_STAGE_ORDER is the display order for PRs someone asked me to review.
// Actionable stages come first; "authored" is not here because My PRs render as
// their own section with different affordances.
export const REQUESTED_STAGE_ORDER: ReviewStage[] = [
  "awaiting_first",
  "re_review",
  "approved",
  "reviewed_waiting",
  "terminal",
];

// reviewStage classifies one review item. Rule order is load-bearing: a PR that
// others already approved but that still awaits MY first look belongs in
// awaiting_first, not approved — it is still my turn.
export function reviewStage(r: ReviewRequest): ReviewStage {
  if (reviewStatusMeta(r.reviewStatus).terminal) return "terminal";
  if (r.kind === "authored") return "authored";
  if (r.reviewStatus === "re_review_required") return "re_review";
  if (!r.reviewedAt && !r.myReviewState) return "awaiting_first";
  if (r.reviewStatus === "approved") return "approved";
  return "reviewed_waiting";
}

export interface StageMeta {
  // title heads the section in the reviews list.
  title: string;
  // badge is the chip rendered on the card; "" defers to reviewStatusMeta so
  // terminal and authored PRs keep their existing Merged/Closed/decision badges.
  badge: string;
  // className is appended to "pr-status-badge".
  className: string;
}

export function stageMeta(stage: ReviewStage): StageMeta {
  switch (stage) {
    case "awaiting_first":
      return { title: "Awaiting your first review", badge: "Review Required", className: " pr-status--awaiting-first" };
    case "re_review":
      return { title: "Re-review requested", badge: "Re-review requested", className: " pr-status--re-review" };
    case "approved":
      return { title: "Approved", badge: "Approved", className: " pr-status--approved" };
    case "reviewed_waiting":
      return { title: "Reviewed — waiting for author", badge: "Reviewed by you", className: " pr-status--waiting" };
    case "terminal":
      return { title: "Merged / Closed", badge: "", className: "" };
    case "authored":
      return { title: "My PRs", badge: "", className: "" };
  }
}

// myReviewLabel renders my own standing decision as a short chip, or null when I
// hold none. It is what makes "approved by me" visible even though the Approved
// bucket keys off the PR-wide decision.
export function myReviewLabel(r: ReviewRequest): string | null {
  switch (r.myReviewState) {
    case "approved":
      return "You: approved";
    case "changes_requested":
      return "You: changes requested";
    case "commented":
      return "You: commented";
    default:
      return null;
  }
}
