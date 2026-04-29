/**
 * Classifies a captured text string as plain text, a Jira issue URL,
 * or a VCS branch/PR URL, and extracts the structured data needed to
 * attach it to a context without manual copy-pasting.
 *
 * Only applied when the entire trimmed text is a single URL (no whitespace).
 * Paragraphs or multi-line selections that happen to contain a link fall
 * through to the generic "text" type.
 */

export type AttachKind =
  | { type: "text" }
  | { type: "jira"; key: string; baseUrl: string }
  | { type: "vcs_branch"; platform: "github" | "gitlab"; ownerRepo: string; branch: string }
  | { type: "vcs_pr"; platform: "github" | "gitlab"; ownerRepo: string; prNumber: number };

// Jira issue key: one or more uppercase letters (project prefix), dash, digits.
const JIRA_KEY_RE = /^[A-Z][A-Z0-9]+-\d+$/;

/**
 * Parses `text` and returns a structured `AttachKind`.
 * Matching is only attempted when the trimmed text contains no whitespace
 * (i.e. it is a single token / URL).
 */
export function parseAttachableContent(text: string): AttachKind {
  const trimmed = text.trim();

  // Multi-word selection → always plain text.
  if (!trimmed || /\s/.test(trimmed)) {
    return { type: "text" };
  }

  let url: URL;
  try {
    url = new URL(trimmed);
    if (url.protocol !== "https:" && url.protocol !== "http:") {
      return { type: "text" };
    }
  } catch {
    // Maybe the user selected a URL without the protocol (e.g. "github.com/org/repo/pull/1").
    // Try prepending https:// and re-parse.
    try {
      url = new URL("https://" + trimmed);
      // Sanity-check: must look like a real host (contains a dot, no spaces).
      if (!url.hostname.includes(".")) return { type: "text" };
    } catch {
      return { type: "text" };
    }
  }

  const host = url.hostname.toLowerCase();
  const pathname = url.pathname;

  // ── Jira ──────────────────────────────────────────────────────────────────
  // https://<anything>.atlassian.net/browse/PROJ-123
  // https://<custom-domain>/browse/PROJ-123
  const jiraMatch = /^\/browse\/([^/?#]+)/.exec(pathname);
  if (jiraMatch) {
    const candidate = jiraMatch[1];
    if (JIRA_KEY_RE.test(candidate)) {
      // baseUrl = scheme + host (+ optional port), no trailing slash.
      const baseUrl = `${url.protocol}//${url.host}`;
      return { type: "jira", key: candidate, baseUrl };
    }
  }

  // ── GitHub ────────────────────────────────────────────────────────────────
  if (host === "github.com") {
    // /owner/repo/tree/branch-or-path (branch may contain slashes)
    const branchMatch = /^\/([^/]+)\/([^/]+)\/tree\/(.+)$/.exec(pathname);
    if (branchMatch) {
      const ownerRepo = `${branchMatch[1]}/${branchMatch[2]}`;
      const branch = branchMatch[3];
      return { type: "vcs_branch", platform: "github", ownerRepo, branch };
    }

    // /owner/repo/pull/123  — fetch this specific PR and resolve its head branch.
    const prMatch = /^\/([^/]+)\/([^/]+)\/pull\/(\d+)/.exec(pathname);
    if (prMatch) {
      const ownerRepo = `${prMatch[1]}/${prMatch[2]}`;
      const prNumber = parseInt(prMatch[3], 10);
      return { type: "vcs_pr", platform: "github", ownerRepo, prNumber };
    }
  }

  // ── GitLab ────────────────────────────────────────────────────────────────
  if (host === "gitlab.com") {
    // /owner/repo/-/tree/branch-or-path
    const branchMatch = /^\/([^/]+)\/([^/]+)\/-\/tree\/(.+)$/.exec(pathname);
    if (branchMatch) {
      const ownerRepo = `${branchMatch[1]}/${branchMatch[2]}`;
      const branch = branchMatch[3];
      return { type: "vcs_branch", platform: "gitlab", ownerRepo, branch };
    }

    // /owner/repo/-/merge_requests/55  — fetch this specific MR.
    const mrMatch = /^\/([^/]+)\/([^/]+)\/-\/merge_requests\/(\d+)/.exec(pathname);
    if (mrMatch) {
      const ownerRepo = `${mrMatch[1]}/${mrMatch[2]}`;
      const prNumber = parseInt(mrMatch[3], 10);
      return { type: "vcs_pr", platform: "gitlab", ownerRepo, prNumber };
    }
  }

  return { type: "text" };
}
