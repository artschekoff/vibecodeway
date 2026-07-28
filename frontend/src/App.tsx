import { useState, useRef, useEffect, useCallback } from "react";
import { TabProvider, useTabContext } from "./contexts/TabContext";
import { PassphraseProvider } from "./contexts/PassphraseContext";
import { TabContent } from "./components/TabContent";
import { EditorFilesProvider } from "./contexts/EditorFilesContext";
import { EditorFileTabBar } from "./components/editor/EditorFileTabBar";
import HamburgerMenu from "./components/HamburgerMenu";
import TabBar from "./components/TabBar";
import { ListAllContexts, ListOrganizations } from "../wailsjs/go/main/App";
import { ProgressProvider } from "./contexts/ProgressContext";
import ProgressModal from "./components/ProgressModal";
import { QuickAttachOverlay } from "./components/QuickAttachOverlay";
import { AccessibilityBanner } from "./components/AccessibilityBanner";
import { BgRefreshToast } from "./components/BgRefreshToast";
import { EventsOn, EventsOff, SendNotification, InitializeNotifications } from "../wailsjs/runtime/runtime";
import { hookNotificationOptions, type HookNotifyPayload } from "./lib/hookNotify";
import "./style.css";

const isWailsContext = () =>
  typeof (window as unknown as Record<string, unknown>)["go"] !== "undefined";

const safeOff = (off: unknown, event: string) => {
  if (typeof off === "function") (off as () => void)();
  else EventsOff(event);
};

// ─── Tab content renderer ────────────────────────────────────────────────────
// TabContent (keep-alive editor tabs) lives in ./components/TabContent.

// ─── Main app shell ──────────────────────────────────────────────────────────

interface PickerState { visible: boolean; text: string }

function AppShell() {
  const [picker, setPicker] = useState<PickerState>({ visible: false, text: "" });
  const [showA11yBanner, setShowA11yBanner] = useState(false);
  const [toastMsg, setToastMsg] = useState("");
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { openContextTab, tabs, activeTabId, forceCloseTab } = useTabContext();
  const activeTab = tabs.find((t) => t.id === activeTabId);
  const activeOrgId = activeTab?.type === "org" ? activeTab.orgId : undefined;

  const showToast = useCallback((msg: string) => {
    setToastMsg(msg);
    if (toastTimer.current) clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => setToastMsg(""), 3000);
  }, []);

  useEffect(() => () => { if (toastTimer.current) clearTimeout(toastTimer.current); }, []);

  useEffect(() => {
    if (!isWailsContext()) return;
    InitializeNotifications().catch(() => {});
    const offPicker = EventsOn("hotkey:show-picker", (text: string) =>
      setPicker({ visible: true, text: text ?? "" }));
    const offA11y = EventsOn("accessibility:needed", () => setShowA11yBanner(true));
    const offA11yGranted = EventsOn("accessibility:granted", () => setShowA11yBanner(false));
    const offNoSelection = EventsOn("hotkey:no-selection", () =>
      showToast("No text selected — select text in another app first"));
    const offHookNotify = EventsOn("hook:notify", (payload: HookNotifyPayload) => {
      SendNotification(hookNotificationOptions(payload)).catch(() => {});
    });
    const offAutoLink = EventsOn("autolink:done", (payload: { contextId?: string; added?: number }) => {
      if (payload?.added && payload.added > 0) {
        showToast(`Auto-linked ${payload.added} repo${payload.added === 1 ? "" : "s"}`);
        if (payload.contextId) {
          window.dispatchEvent(new CustomEvent("context:updated", { detail: { id: payload.contextId } }));
        }
      }
    });
    return () => {
      safeOff(offPicker, "hotkey:show-picker");
      safeOff(offA11y, "accessibility:needed");
      safeOff(offA11yGranted, "accessibility:granted");
      safeOff(offNoSelection, "hotkey:no-selection");
      safeOff(offHookNotify, "hook:notify");
      safeOff(offAutoLink, "autolink:done");
    };
  }, [showToast]);

  // On mount: validate restored tabs against live DB to close stale references
  useEffect(() => {
    if (!isWailsContext()) return;
    void (async () => {
      try {
        const [allContexts, allOrgs] = await Promise.all([ListAllContexts(), ListOrganizations()]);
        const validContextIds = new Set((allContexts ?? []).map((c) => c.id));
        const validOrgIds = new Set((allOrgs ?? []).map((o) => o.id));
        for (const tab of tabs) {
          if (tab.type === "context" && tab.contextId && !validContextIds.has(tab.contextId)) {
            forceCloseTab(tab.id);
          }
          if (tab.type === "org" && tab.orgId && !validOrgIds.has(tab.orgId)) {
            forceCloseTab(tab.id);
          }
        }
      } catch {
        // best-effort — if the call fails, leave tabs as-is
      }
    })();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []); // intentionally run once on mount only

  const handleAttach = useCallback(
    (contextKey: string, contextId: string, orgId?: string, orgName?: string) => {
      setPicker({ visible: false, text: "" });
      showToast(`Attached to "${contextKey}"`);
      window.dispatchEvent(new CustomEvent("context:updated", { detail: { id: contextId } }));
      openContextTab(contextId, contextKey, orgName, orgId);
      SendNotification({
        id: `quick-attach-${Date.now()}`,
        title: "Attached",
        body: `Added to context: ${contextKey}`,
      }).catch(() => {});
    },
    [showToast, openContextTab]
  );

  return (
    <ProgressProvider>
      <ProgressModal />
      <div className="app-shell">
        <div className="app-sidebar">
          <HamburgerMenu />
          <TabBar />
        </div>
        <div className="app-content">
          {activeTab?.type === "editor" && activeTab.contextId && (
            <EditorFileTabBar contextId={activeTab.contextId} />
          )}
          <TabContent />
        </div>
      </div>
      {showA11yBanner && <AccessibilityBanner onDismiss={() => setShowA11yBanner(false)} />}
      <QuickAttachOverlay
        visible={picker.visible}
        text={picker.text}
        orgId={activeOrgId}
        onAttach={handleAttach}
        onDismiss={() => setPicker({ visible: false, text: "" })}
      />
      {toastMsg && <div className="toast toast--success">{toastMsg}</div>}
      <BgRefreshToast />
    </ProgressProvider>
  );
}

// ─── Root ─────────────────────────────────────────────────────────────────────

function App() {
  return (
    <PassphraseProvider>
      <TabProvider>
        <EditorFilesProvider>
          <AppShell />
        </EditorFilesProvider>
      </TabProvider>
    </PassphraseProvider>
  );
}

export default App;
