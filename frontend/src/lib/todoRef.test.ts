import { describe, it, expect } from "vitest";
import { todoRefUrl } from "./todoRef";

describe("todoRefUrl", () => {
  it("builds a Jira browse URL from a jira ref", () => {
    expect(todoRefUrl("jira", "JH-4922", "https://acme.atlassian.net")).toBe(
      "https://acme.atlassian.net/browse/JH-4922",
    );
  });

  it("tolerates a trailing slash on the Jira base URL", () => {
    expect(todoRefUrl("jira", "JH-1", "https://acme.atlassian.net/")).toBe(
      "https://acme.atlassian.net/browse/JH-1",
    );
  });

  it("returns empty for a jira ref when no base URL is configured", () => {
    expect(todoRefUrl("jira", "JH-4922", "")).toBe("");
  });

  it("builds a GitHub PR URL from a pr ref", () => {
    expect(todoRefUrl("pr", "owner/repo#218", "")).toBe(
      "https://github.com/owner/repo/pull/218",
    );
  });

  it("treats a review ref the same as a pr ref", () => {
    expect(todoRefUrl("review", "o/r#42", "")).toBe(
      "https://github.com/o/r/pull/42",
    );
  });

  it("trims whitespace around the refKey", () => {
    expect(todoRefUrl("pr", "  o/r#42  ", "")).toBe(
      "https://github.com/o/r/pull/42",
    );
  });

  it("returns empty for an empty refKey", () => {
    expect(todoRefUrl("pr", "", "")).toBe("");
  });

  it("returns empty for a malformed pr ref", () => {
    expect(todoRefUrl("pr", "not-a-ref", "")).toBe("");
  });

  it("returns empty for an unknown refType", () => {
    expect(todoRefUrl("", "JH-1", "https://acme.atlassian.net")).toBe("");
  });

  it("returns empty (does not throw) when all args are undefined", () => {
    expect(todoRefUrl(undefined, undefined, "")).toBe("");
  });

  it("returns empty (does not throw) for a pr ref with an undefined refKey", () => {
    expect(todoRefUrl("pr", undefined, "")).toBe("");
  });

  it("returns empty for a jira ref with an undefined base URL", () => {
    expect(todoRefUrl("jira", "JH-1", undefined)).toBe("");
  });
});
