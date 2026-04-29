import { describe, it, expect } from "vitest";
import { parseAttachableContent } from "./parseAttachableContent";

describe("parseAttachableContent", () => {
  // ── Plain text ──────────────────────────────────────────────────────────
  it("returns text for a plain sentence", () => {
    expect(parseAttachableContent("some selected text here")).toEqual({ type: "text" });
  });

  it("returns text for empty string", () => {
    expect(parseAttachableContent("")).toEqual({ type: "text" });
  });

  it("returns text for whitespace only", () => {
    expect(parseAttachableContent("   ")).toEqual({ type: "text" });
  });

  it("returns text when a URL is embedded in a sentence", () => {
    expect(parseAttachableContent("Check this https://github.com/org/repo/tree/main out")).toEqual({ type: "text" });
  });

  it("returns text for a non-http URL scheme", () => {
    expect(parseAttachableContent("ftp://example.com/browse/PROJ-1")).toEqual({ type: "text" });
  });

  // ── Jira ────────────────────────────────────────────────────────────────
  it("parses an atlassian.net Jira issue URL", () => {
    const result = parseAttachableContent("https://acme.atlassian.net/browse/PROJ-123");
    expect(result).toEqual({
      type: "jira",
      key: "PROJ-123",
      baseUrl: "https://acme.atlassian.net",
    });
  });

  it("handles leading/trailing whitespace in Jira URL", () => {
    const result = parseAttachableContent("  https://acme.atlassian.net/browse/JH-42  ");
    expect(result).toEqual({
      type: "jira",
      key: "JH-42",
      baseUrl: "https://acme.atlassian.net",
    });
  });

  it("parses a custom-domain Jira URL", () => {
    const result = parseAttachableContent("https://jira.mycompany.io/browse/BACK-7");
    expect(result).toEqual({
      type: "jira",
      key: "BACK-7",
      baseUrl: "https://jira.mycompany.io",
    });
  });

  it("returns text for /browse/ without a valid Jira key", () => {
    expect(parseAttachableContent("https://acme.atlassian.net/browse/not-a-key")).toEqual({ type: "text" });
    expect(parseAttachableContent("https://acme.atlassian.net/browse/123")).toEqual({ type: "text" });
  });

  it("preserves the base URL port if present", () => {
    const result = parseAttachableContent("https://jira.local:8080/browse/DEV-1");
    expect(result).toEqual({
      type: "jira",
      key: "DEV-1",
      baseUrl: "https://jira.local:8080",
    });
  });

  // ── GitHub branch ────────────────────────────────────────────────────────
  it("parses a GitHub /tree/ URL", () => {
    const result = parseAttachableContent("https://github.com/acme/my-service/tree/feature/new-ui");
    expect(result).toEqual({
      type: "vcs_branch",
      platform: "github",
      ownerRepo: "acme/my-service",
      branch: "feature/new-ui",
    });
  });

  it("parses a simple GitHub branch (no slash in branch name)", () => {
    const result = parseAttachableContent("https://github.com/org/repo/tree/main");
    expect(result).toEqual({
      type: "vcs_branch",
      platform: "github",
      ownerRepo: "org/repo",
      branch: "main",
    });
  });

  // ── GitHub PR ────────────────────────────────────────────────────────────
  it("parses a GitHub PR URL as vcs_pr", () => {
    const result = parseAttachableContent("https://github.com/org/repo/pull/203");
    expect(result).toEqual({
      type: "vcs_pr",
      platform: "github",
      ownerRepo: "org/repo",
      prNumber: 203,
    });
  });

  it("parses a GitHub PR URL with trailing slash as vcs_pr", () => {
    const result = parseAttachableContent("https://github.com/org/repo/pull/5/");
    expect(result).toEqual({
      type: "vcs_pr",
      platform: "github",
      ownerRepo: "org/repo",
      prNumber: 5,
    });
  });

  it("parses a GitHub PR URL with a subpath (e.g. /files) as vcs_pr", () => {
    const result = parseAttachableContent("https://github.com/org/repo/pull/7/files");
    expect(result).toEqual({
      type: "vcs_pr",
      platform: "github",
      ownerRepo: "org/repo",
      prNumber: 7,
    });
  });

  // ── GitLab branch ────────────────────────────────────────────────────────
  it("parses a GitLab /-/tree/ URL", () => {
    const result = parseAttachableContent("https://gitlab.com/group/project/-/tree/feat/some-feature");
    expect(result).toEqual({
      type: "vcs_branch",
      platform: "gitlab",
      ownerRepo: "group/project",
      branch: "feat/some-feature",
    });
  });

  it("parses a GitLab merge request URL as vcs_pr", () => {
    const result = parseAttachableContent("https://gitlab.com/group/project/-/merge_requests/55");
    expect(result).toEqual({
      type: "vcs_pr",
      platform: "gitlab",
      ownerRepo: "group/project",
      prNumber: 55,
    });
  });

  // ── Non-matching URLs fall through to text ───────────────────────────────
  it("returns text for a random https URL", () => {
    expect(parseAttachableContent("https://example.com/some/page")).toEqual({ type: "text" });
  });

  it("returns text for a GitHub URL that is not a branch or PR", () => {
    expect(parseAttachableContent("https://github.com/org/repo")).toEqual({ type: "text" });
    expect(parseAttachableContent("https://github.com/org/repo/issues/5")).toEqual({ type: "text" });
  });
});
