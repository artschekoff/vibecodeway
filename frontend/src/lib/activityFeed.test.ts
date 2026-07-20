import { describe, it, expect } from "vitest";
import { activitySource, describeActivity } from "./activityFeed";
import type { ActivityFeedItem } from "../types";

function item(partial: Partial<ActivityFeedItem>): ActivityFeedItem {
  return {
    id: "n1",
    contextId: "c1",
    contextKey: "PROJ-1",
    entityType: "pull_request",
    eventType: "pr_merged",
    oldValue: "",
    newValue: "",
    createdAt: "2026-07-20T10:00:00Z",
    ...partial,
  };
}

describe("activitySource", () => {
  it("maps jira_issue to jira", () => {
    expect(activitySource(item({ entityType: "jira_issue" }))).toBe("jira");
  });
  it("maps pull_request (and anything else) to github", () => {
    expect(activitySource(item({ entityType: "pull_request" }))).toBe("github");
    expect(activitySource(item({ entityType: "something_else" }))).toBe("github");
  });
});

describe("describeActivity", () => {
  it("describes a jira status change with old and new values", () => {
    expect(
      describeActivity(item({ eventType: "jira_status_changed", oldValue: "To Do", newValue: "Done" })),
    ).toBe("Jira status: To Do → Done");
  });
  it("describes known PR events", () => {
    expect(describeActivity(item({ eventType: "pr_merged" }))).toBe("PR merged");
    expect(describeActivity(item({ eventType: "ready_to_merge" }))).toBe("Ready to merge");
    expect(describeActivity(item({ eventType: "ci_failure" }))).toBe("CI failed");
  });
  it("falls back to a humanized event type for unknown events", () => {
    expect(describeActivity(item({ eventType: "some_new_event" }))).toBe("some new event");
  });
});
