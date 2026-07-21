import { useEffect, useState } from "react";
import {
  ListContextEventHooks,
  CreateContextEventHook,
  DeleteContextEventHook,
} from "../../wailsjs/go/main/App";
import type { ContextEventHook, EventCategory, HookActionType } from "../types";
import EventHooksEditor from "./EventHooksEditor";

export default function ContextEventHooks({ contextId }: { contextId: string }) {
  const [hooks, setHooks] = useState<ContextEventHook[]>([]);

  const load = () => {
    void ListContextEventHooks(contextId)
      .then((res) => setHooks((res ?? []) as unknown as ContextEventHook[]))
      .catch(() => setHooks([]));
  };
  useEffect(load, [contextId]);

  return (
    <EventHooksEditor
      title="Event Hooks"
      hooks={hooks}
      onAdd={async (event: EventCategory, action: HookActionType, script: string) => {
        await CreateContextEventHook(contextId, event, action, script);
        load();
      }}
      onDelete={async (id: string) => {
        await DeleteContextEventHook(id);
        load();
      }}
    />
  );
}
