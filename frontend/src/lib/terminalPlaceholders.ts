/**
 * Replaces angle-bracket placeholder tokens (e.g. <context-name>) in a terminal
 * command template with values from `vars`. Only tokens present in `vars` are
 * replaced; unknown tokens are left as-is. Add new tokens by passing more keys.
 */
export function applyPlaceholders(template: string, vars: Record<string, string>): string {
  return template.replace(/<([a-z0-9-]+)>/gi, (match, key: string) =>
    Object.prototype.hasOwnProperty.call(vars, key) ? vars[key] : match
  );
}

// shellQuote wraps text as ONE double-quoted shell argument, escaping the
// characters both fish and bash treat specially inside double quotes.
// ponytail: backticks are left alone (fish ignores them; the hydration prompt
// avoids them) — full per-shell quoting isn't worth it here.
function shellQuote(s: string): string {
  return '"' + s.replace(/([\\"$])/g, "\\$1") + '"';
}

// withAgentHydration builds the final inserted command by substituting the
// explicit <context-hydration> placeholder with the shell-quoted hydration
// prompt. No magic agent detection: a command opts in by carrying the
// placeholder (migration 126 rewrote known agent launches to do so), which
// keeps the behavior visible and editable in Settings -> Terminal commands.
export function withAgentHydration(command: string, hydration: string): string {
  if (!hydration) return command.replace(/<context-hydration>/gi, "");
  return command.replace(/<context-hydration>/gi, shellQuote(hydration));
}
