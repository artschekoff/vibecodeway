import { useEffect, useState } from "react";
import {
  ListGlobalEventHooks,
  CreateGlobalEventHook,
  DeleteGlobalEventHook,
} from "../../wailsjs/go/main/App";
import type { GlobalEventHook, EventCategory, HookActionType } from "../types";
import EventHooksEditor from "./EventHooksEditor";

export default function GlobalEventHooks() {
  const [hooks, setHooks] = useState<GlobalEventHook[]>([]);

  const load = () => {
    void ListGlobalEventHooks()
      .then((res) => setHooks((res ?? []) as unknown as GlobalEventHook[]))
      .catch(() => setHooks([]));
  };
  useEffect(load, []);

  return (
    <EventHooksEditor
      title="Global Event Hooks (default for all contexts)"
      hooks={hooks}
      onAdd={async (event: EventCategory, action: HookActionType, script: string) => {
        await CreateGlobalEventHook(event, action, script);
        load();
      }}
      onDelete={async (id: string) => {
        await DeleteGlobalEventHook(id);
        load();
      }}
    />
  );
}
