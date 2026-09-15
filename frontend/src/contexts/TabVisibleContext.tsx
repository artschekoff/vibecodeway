import { createContext, useContext } from "react";

/**
 * Whether the enclosing app tab is the visible (active) one. Editor tabs stay
 * mounted with display:none when inactive (TabContent), so every hidden tab's
 * hotkey handlers are still registered — gate them with this so ⌘I/⌘T/etc.
 * act on the tab the user is looking at, not the first-mounted hidden one.
 * Defaults to true for trees not rendered inside a tab (tests, modals).
 */
export const TabVisibleContext = createContext(true);

export function useTabVisible(): boolean {
  return useContext(TabVisibleContext);
}
