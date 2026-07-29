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
  return { commands, reload };
}
