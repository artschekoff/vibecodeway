import { useEffect, useState } from "react";
import {
  ListContextEventHooks,
  CreateContextEventHook,
  DeleteContextEventHook,
} from "../../wailsjs/go/main/App";
import type { ContextEventHook, EventCategory, HookActionType } from "../types";

const EVENT_LABELS: Record<EventCategory, string> = {
  jira_update: "Jira ticket update",
  pr_update: "GitHub PR update",
  code_review: "New code review",
};
const ACTION_LABELS: Record<HookActionType, string> = {
  os_notification: "OS notification",
  run_script: "Run script",
};

export default function ContextEventHooks({ contextId }: { contextId: string }) {
  const [hooks, setHooks] = useState<ContextEventHook[]>([]);
  const [event, setEvent] = useState<EventCategory>("pr_update");
  const [action, setAction] = useState<HookActionType>("os_notification");
  const [script, setScript] = useState("");

  const load = () => {
    void ListContextEventHooks(contextId)
      .then((res) => setHooks((res ?? []) as unknown as ContextEventHook[]))
      .catch(() => setHooks([]));
  };
  useEffect(load, [contextId]);

  const add = async () => {
    await CreateContextEventHook(contextId, event, action, action === "run_script" ? script : "");
    setScript("");
    load();
  };
  const remove = async (id: string) => {
    await DeleteContextEventHook(id);
    load();
  };

  return (
    <section className="detail-section" style={{ marginBottom: 12 }}>
      <div className="detail-section-header">
        <span className="detail-section-title">Event Hooks</span>
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: 10, padding: "12px 14px" }}>
        {hooks.length === 0 && (
          <p style={{ color: "var(--text-muted)", margin: 0 }}>No hooks configured.</p>
        )}
        {hooks.map((h) => (
          <div key={h.id} style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <span style={{ flex: 1 }}>
              {EVENT_LABELS[h.eventCategory]} → {ACTION_LABELS[h.actionType]}
              {h.actionType === "run_script" && h.scriptCommand ? `: ${h.scriptCommand}` : ""}
            </span>
            <button className="vcs-checkout-btn vcs-checkout-btn--danger" onClick={() => void remove(h.id)}>
              Delete
            </button>
          </div>
        ))}

        <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
          <select
            data-testid="hook-event"
            className="passphrase-input"
            value={event}
            onChange={(e) => setEvent(e.target.value as EventCategory)}
          >
            <option value="pr_update">GitHub PR update</option>
            <option value="jira_update">Jira ticket update</option>
            <option value="code_review">New code review</option>
          </select>
          <select
            data-testid="hook-action"
            className="passphrase-input"
            value={action}
            onChange={(e) => setAction(e.target.value as HookActionType)}
          >
            <option value="os_notification">OS notification</option>
            <option value="run_script">Run script</option>
          </select>
          {action === "run_script" && (
            <input
              data-testid="hook-script"
              className="passphrase-input"
              placeholder="shell command ($CONTEXT_KEY available)"
              value={script}
              onChange={(e) => setScript(e.target.value)}
            />
          )}
          <button className="btn-primary" onClick={() => void add()}>Add hook</button>
        </div>
      </div>
    </section>
  );
}
