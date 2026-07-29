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
