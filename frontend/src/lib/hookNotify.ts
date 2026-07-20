export interface HookNotifyPayload {
  contextId: string;
  contextKey: string;
  eventCategory: string;
}

const CATEGORY_TEXT: Record<string, string> = {
  jira_update: "Jira update",
  pr_update: "GitHub PR update",
  code_review: "new code review",
};

export function hookNotificationOptions(p: HookNotifyPayload): { id: string; title: string; body: string } {
  const what = CATEGORY_TEXT[p.eventCategory] ?? "new event";
  return {
    id: `hook-${p.contextId}-${p.eventCategory}`,
    title: `New event in ${p.contextKey}`,
    body: `${what} in context ${p.contextKey}`,
  };
}
