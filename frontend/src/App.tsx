import { useState, useRef, useEffect, useCallback } from "react";
import { TabProvider, useTabContext } from "./contexts/TabContext";
import { PassphraseProvider } from "./contexts/PassphraseContext";
import ContextList from "./pages/ContextList";
import ContextDetail from "./pages/ContextDetail";
import Settings from "./pages/Settings";
import AppLogsPage from "./pages/AppLogsPage";
import PromptsPage from "./pages/PromptsPage";
import MyBoardPage from "./pages/MyBoardPage";
import AskAIPage from "./pages/AskAIPage";
import HamburgerMenu from "./components/HamburgerMenu";
import TabBar from "./components/TabBar";
import { ListAllContexts, ListOrganizations } from "../wailsjs/go/main/App";
import { ProgressProvider } from "./contexts/ProgressContext";
import ProgressModal from "./components/ProgressModal";
import { QuickAttachOverlay } from "./components/QuickAttachOverlay";
import { AccessibilityBanner } from "./components/AccessibilityBanner";
import { BgRefreshToast } from "./components/BgRefreshToast";
import { EventsOn, EventsOff, SendNotification, InitializeNotifications } from "../wailsjs/runtime/runtime";
import "./style.css";

const isWailsContext = () =>
  typeof (window as unknown as Record<string, unknown>)["go"] !== "undefined";

const safeOff = (off: unknown, event: string) => {
  if (typeof off === "function") (off as () => void)();
  else EventsOff(event);
};

// ─── Tab content renderer ────────────────────────────────────────────────────

function TabContent() {
  const { tabs, activeTabId } = useTabContext();
  const activeTab = tabs.find((t) => t.id === activeTabId);

  if (!activeTab) {
    return (
      <div className="empty-state" style={{ margin: "auto", padding: "40px", textAlign: "center" }}>
        <p style={{ color: "var(--text-muted)" }}>Open an organization from the ☰ menu to get started.</p>
      </div>
    );
  }

  if (activeTab.type === "settings") return <Settings />;
  if (activeTab.type === "logs") return <AppLogsPage />;
  if (activeTab.type === "prompts") return <PromptsPage />;
  if (activeTab.type === "board") return <MyBoardPage />;
  if (activeTab.type === "ask") return <AskAIPage />;
  if (activeTab.type === "org") {
    return <ContextList orgId={activeTab.orgId!} tabId={activeTab.id} />;
  }
  if (activeTab.type === "context") {
    return <ContextDetail contextId={activeTab.contextId!} tabId={activeTab.id} />;
  }
  return null;
}

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
    return () => {
      safeOff(offPicker, "hotkey:show-picker");
      safeOff(offA11y, "accessibility:needed");
      safeOff(offA11yGranted, "accessibility:granted");
      safeOff(offNoSelection, "hotkey:no-selection");
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
        <AppShell />
      </TabProvider>
    </PassphraseProvider>
  );
}

export default App;
