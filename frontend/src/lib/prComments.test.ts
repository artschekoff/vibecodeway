import { describe, it, expect } from "vitest";
import { countUnreadComments } from "./prComments";
import type { PRComment } from "../types";

const c = (over: Partial<PRComment>): PRComment => ({
  id: "x", prId: "p", externalId: "e", commentType: "review", author: "a", body: "b",
  isResolved: false, commentCreatedAt: "", commentUpdatedAt: "", ...over,
}) as PRComment;

describe("countUnreadComments", () => {
  it("returns 0 for empty or undefined", () => {
    expect(countUnreadComments(undefined)).toBe(0);
    expect(countUnreadComments([])).toBe(0);
  });

  it("counts only comments without viewedAt, including nested replies", () => {
    const comments: PRComment[] = [
      c({ id: "1", viewedAt: "2026-01-01" }),           // read
      c({ id: "2" }),                                    // unread
      c({ id: "3", replies: [c({ id: "4" }), c({ id: "5", viewedAt: "x" })] }), // unread + 1 unread reply + 1 read reply
    ];
    expect(countUnreadComments(comments)).toBe(3);
  });
});
