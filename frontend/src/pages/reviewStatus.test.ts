import { describe, expect, it } from "vitest";
import { reviewStatusMeta } from "./reviewStatus";

describe("reviewStatusMeta", () => {
  it("flags re_review_required with a re-approval label", () => {
    const meta = reviewStatusMeta("re_review_required");
    expect(meta.needsReReview).toBe(true);
    expect(meta.label).toBe("Re-approval Needed");
    expect(meta.className).toBe(" pr-status--re-review");
  });

  it("maps the existing statuses unchanged", () => {
    expect(reviewStatusMeta("changes_requested")).toEqual({
      label: "Changes Requested",
      className: " pr-status--changes-requested",
      needsReReview: false,
      terminal: false,
    });
    expect(reviewStatusMeta("approved")).toEqual({
      label: "Approved",
      className: " pr-status--approved",
      needsReReview: false,
      terminal: false,
    });
    expect(reviewStatusMeta("review_required")).toEqual({
      label: "Review Required",
      className: "",
      needsReReview: false,
      terminal: false,
    });
  });

  it("returns a null label for unknown statuses", () => {
    expect(reviewStatusMeta("")).toEqual({ label: null, className: "", needsReReview: false, terminal: false });
  });
});

describe("reviewStatusMeta — terminal states", () => {
  it("labels merged with a terminal flag", () => {
    expect(reviewStatusMeta("merged")).toEqual({
      label: "Merged",
      className: " pr-status--merged",
      needsReReview: false,
      terminal: true,
    });
  });

  it("labels closed with a terminal flag", () => {
    expect(reviewStatusMeta("closed")).toEqual({
      label: "Closed",
      className: " pr-status--closed",
      needsReReview: false,
      terminal: true,
    });
  });

  it("marks non-terminal statuses terminal:false", () => {
    expect(reviewStatusMeta("approved").terminal).toBe(false);
    expect(reviewStatusMeta("re_review_required").terminal).toBe(false);
    expect(reviewStatusMeta("").terminal).toBe(false);
  });
});
