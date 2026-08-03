import { useCallback, useEffect, useState } from "react";
import { ListTerminalCommands } from "../../wailsjs/go/main/App";
import type { store } from "../../wailsjs/go/models";

/** Loads user-configured terminal commands for the terminal command selector. */
export function useTerminalCommands(): { commands: store.TerminalCommand[]; reload: () => void } {
  const [commands, setCommands] = useState<store.TerminalCommand[]>([]);
  const reload = useCallback(() => {
    void ListTerminalCommands().then((rows) => setCommands(rows ?? [])).catch(() => setCommands([]));
  }, []);
  useEffect(() => { reload(); }, [reload]);
  // Editor tabs stay mounted (display:none), so a snippet edited in Settings
  // afterward would otherwise be stale here. Settings fires this event on change.
  useEffect(() => {
    const onChanged = () => reload();
    window.addEventListener("terminal-commands-changed", onChanged);
    return () => window.removeEventListener("terminal-commands-changed", onChanged);
  }, [reload]);
  return { commands, reload };
}
