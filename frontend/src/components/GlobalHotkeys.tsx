import { useHotkey } from "../hotkeys/HotkeyProvider";
import { useTabContext } from "../contexts/TabContext";
import { GetVCSPRsForContext, RefreshContextPRs } from "../../wailsjs/go/main/App";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";

/**
 * Registers shell-level (global scope) hotkey handlers that don't own a
 * dedicated component: tab switching, and context/PR commands that act on the
 * currently-active context tab. Mounted once inside TabProvider.
 */
export function GlobalHotkeys() {
  const { tabs, activeTabId, focusNextTab, focusPrevTab, closeTab } = useTabContext();
  const active = tabs.find((t) => t.id === activeTabId);
  const contextId = active?.contextId;
  const contextKey = active?.contextKey;

  useHotkey("tabs.next", focusNextTab, { scope: "global" });
  useHotkey("tabs.prev", focusPrevTab, { scope: "global" });

  // ⌘⇧W — close the active context tab. closeTab is a no-op on pinned tabs.
  useHotkey("tabs.close", () => {
    if (activeTabId) closeTab(activeTabId);
  }, { scope: "global" });

  // ⌘C N — copy the active context key (no bare-⌘C binding, so native copy is safe).
  useHotkey("context.copyName", () => {
    if (contextKey) void navigator.clipboard?.writeText(contextKey);
  }, { scope: "global" });

  // ⌘G P — open the active context's first PR in the browser (no-op if none).
  useHotkey("pr.open", () => {
    if (!contextId) return;
    void GetVCSPRsForContext(contextId).then((prs) => {
      const url = prs?.[0]?.url;
      if (url) BrowserOpenURL(url);
    });
  }, { scope: "global" });

  // ⌘P R — re-sync the active context's PR list.
  useHotkey("pr.reload", () => {
    if (contextId) void RefreshContextPRs(contextId);
  }, { scope: "global" });

  return null;
}

export default GlobalHotkeys;
