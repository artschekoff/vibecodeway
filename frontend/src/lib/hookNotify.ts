export interface HookNotifyPayload {
  contextId: string;
  contextKey: string;
  eventCategory: string;
  // Both are set only when a notification rule matched the event.
  conditionText?: string;
  note?: string;
}

const CATEGORY_TEXT: Record<string, string> = {
  jira_update: "Jira update",
  pr_update: "GitHub PR update",
  code_review: "new code review",
};

export function hookNotificationOptions(p: HookNotifyPayload): { id: string; title: string; body: string } {
  const what = CATEGORY_TEXT[p.eventCategory] ?? "new event";
  // A rule match earns a different notification: the user asked to be told about
  // exactly this, so their own note leads and the generic category text goes.
  const lines = [p.conditionText, p.note].filter((s): s is string => !!s && s.length > 0);
  return {
    id: `hook-${p.contextId}-${p.eventCategory}`,
    title: lines.length > 0 ? `${p.contextKey}: condition met` : `New event in ${p.contextKey}`,
    body: lines.length > 0 ? lines.join("\n") : `${what} in context ${p.contextKey}`,
  };
}
