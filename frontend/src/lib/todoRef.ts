/**
 * Build a browser URL for a todo's underlying context from its refType/refKey.
 * Returns "" when there is no resolvable reference so the caller hides the link.
 *
 *   jira:   "JH-4922"        -> `${jiraBaseUrl}/browse/JH-4922`  (needs jiraBaseUrl)
 *   pr:     "owner/repo#218" -> "https://github.com/owner/repo/pull/218"
 *   review: "owner/repo#218" -> same as pr
 */
export function todoRefUrl(refType: string, refKey: string, jiraBaseUrl: string): string {
  const key = refKey.trim();
  if (!key) return "";
  if (refType === "jira") {
    return jiraBaseUrl ? `${jiraBaseUrl.replace(/\/$/, "")}/browse/${key}` : "";
  }
  if (refType === "pr" || refType === "review") {
    const m = key.match(/^([^/\s]+)\/([^#\s]+)#(\d+)$/);
    return m ? `https://github.com/${m[1]}/${m[2]}/pull/${m[3]}` : "";
  }
  return "";
}
