import type { ActivityFeedItem, ActivitySource } from "../types";

export function activitySource(item: ActivityFeedItem): ActivitySource {
  return item.entityType === "jira_issue" ? "jira" : "github";
}

export function describeActivity(item: ActivityFeedItem): string {
  switch (item.eventType) {
    case "jira_status_changed":
      return `Jira status: ${item.oldValue || "?"} → ${item.newValue || "?"}`;
    case "pr_merged":
      return "PR merged";
    case "pr_closed":
      return "PR closed";
    case "pr_conflict":
      return "PR has conflicts";
    case "pr_review_changed":
      return `Review: ${item.newValue || "updated"}`;
    case "ready_to_merge":
      return "Ready to merge";
    case "ci_success":
      return "CI passed";
    case "ci_failure":
      return "CI failed";
    case "ci_pending":
      return "CI running";
    default:
      return item.eventType.replace(/_/g, " ");
  }
}
