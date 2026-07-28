import type { PRComment } from "../types";

/** Count comments (recursively including replies) that have not been viewed. */
export function countUnreadComments(comments?: PRComment[]): number {
  if (!comments) return 0;
  return comments.reduce(
    (n, comment) => n + (comment.viewedAt ? 0 : 1) + countUnreadComments(comment.replies),
    0
  );
}
