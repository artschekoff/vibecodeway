import { describe, it, expect } from "vitest";
import { reviewStage, stageMeta, myReviewLabel, REQUESTED_STAGE_ORDER } from "./reviewStage";
import type { ReviewRequest } from "../types";

function make(over: Partial<ReviewRequest>): ReviewRequest {
  return {
    id: "r1", repoId: "repo1", prNumber: 1, prTitle: "t", prAuthor: "carol",
    prUrl: "https://x/1", reviewStatus: "review_required", requestedAt: "", fetchedAt: "",
    ...over,
  };
}

describe("reviewStage", () => {
  it("classifies a never-reviewed PR as awaiting_first", () => {
    expect(reviewStage(make({}))).toBe("awaiting_first");
  });

  it("keeps a PR approved by others but unreviewed by me in awaiting_first", () => {
    expect(reviewStage(make({ reviewStatus: "approved" }))).toBe("awaiting_first");
  });

  it("classifies a re-requested review as re_review even after I reviewed", () => {
    expect(reviewStage(make({ reviewStatus: "re_review_required", myReviewState: "changes_requested" })))
      .toBe("re_review");
  });

  it("classifies an approved PR I already reviewed as approved", () => {
    expect(reviewStage(make({ reviewStatus: "approved", myReviewState: "approved" }))).toBe("approved");
  });

  it("classifies my changes-requested PR as reviewed_waiting", () => {
    expect(reviewStage(make({ reviewStatus: "changes_requested", myReviewState: "changes_requested" })))
      .toBe("reviewed_waiting");
  });

  it("classifies a PR that dropped off the list as reviewed_waiting", () => {
    expect(reviewStage(make({ reviewedAt: "2026-08-01T00:00:00Z" }))).toBe("reviewed_waiting");
  });

  it("classifies merged and closed PRs as terminal, ahead of every other rule", () => {
    expect(reviewStage(make({ reviewStatus: "merged" }))).toBe("terminal");
    expect(reviewStage(make({ reviewStatus: "closed", kind: "authored" }))).toBe("terminal");
  });

  it("classifies my own open PRs as authored", () => {
    expect(reviewStage(make({ kind: "authored", reviewStatus: "changes_requested" }))).toBe("authored");
  });
});

describe("stageMeta / REQUESTED_STAGE_ORDER", () => {
  it("puts actionable stages first and terminal last", () => {
    expect(REQUESTED_STAGE_ORDER).toEqual([
      "awaiting_first", "re_review", "approved", "reviewed_waiting", "terminal",
    ]);
  });

  it("gives every stage a section title and pins badge/className values", () => {
    expect(stageMeta("awaiting_first")).toEqual({
      title: "Awaiting your first review",
      badge: "Review Required",
      className: " pr-status--awaiting-first",
    });
    expect(stageMeta("re_review")).toEqual({
      title: "Re-review requested",
      badge: "Re-review requested",
      className: " pr-status--re-review",
    });
    expect(stageMeta("approved")).toEqual({
      title: "Approved",
      badge: "Approved",
      className: " pr-status--approved",
    });
    expect(stageMeta("reviewed_waiting")).toEqual({
      title: "Reviewed — waiting for author",
      badge: "Reviewed by you",
      className: " pr-status--waiting",
    });
    expect(stageMeta("terminal").badge).toBe("");
    expect(stageMeta("authored").badge).toBe("");
  });
});

describe("myReviewLabel", () => {
  it("labels my own standing decision", () => {
    expect(myReviewLabel(make({ myReviewState: "approved" }))).toBe("You: approved");
    expect(myReviewLabel(make({ myReviewState: "changes_requested" }))).toBe("You: changes requested");
    expect(myReviewLabel(make({ myReviewState: "commented" }))).toBe("You: commented");
  });

  it("returns null when I have no standing review", () => {
    expect(myReviewLabel(make({}))).toBeNull();
    expect(myReviewLabel(make({ myReviewState: "" }))).toBeNull();
  });
});
