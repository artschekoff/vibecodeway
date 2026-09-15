import { useTabContext } from "../contexts/TabContext";
import type { Tab } from "../types";
import ContextList from "../pages/ContextList";
import ContextDetail from "../pages/ContextDetail";
import Settings from "../pages/Settings";
import AppLogsPage from "../pages/AppLogsPage";
import PromptsPage from "../pages/PromptsPage";
import MyBoardPage from "../pages/MyBoardPage";
import AskAIPage from "../pages/AskAIPage";
import ActivityTimeline from "../pages/ActivityTimeline";
import ActivityFeedPage from "../pages/ActivityFeedPage";
import ProductivityPage from "../pages/ProductivityPage";
import AnalyticsPage from "../pages/AnalyticsPage";
import { EditorWorkspace } from "../pages/EditorWorkspace";
import { TabVisibleContext } from "../contexts/TabVisibleContext";

// Renders the non-editor tab types. Editor tabs are handled separately so they
// can be kept mounted across tab switches.
function NonEditorContent({ tab }: { tab: Tab }) {
  switch (tab.type) {
    case "settings": return <Settings />;
    case "logs": return <AppLogsPage />;
    case "prompts": return <PromptsPage />;
    case "board": return <MyBoardPage />;
    case "ask": return <AskAIPage />;
    case "timeline": return <ActivityTimeline />;
    case "activity-feed": return <ActivityFeedPage />;
    case "productivity": return <ProductivityPage />;
    case "analytics": return <AnalyticsPage />;
    case "org": return <ContextList orgId={tab.orgId!} tabId={tab.id} />;
    case "context": return <ContextDetail contextId={tab.contextId!} tabId={tab.id} />;
    default: return null;
  }
}

/**
 * Renders the active tab's content. Editor tabs are kept mounted while open and
 * merely hidden when inactive, so their terminal (PTY session + xterm buffer)
 * and open files survive tab switches. They unmount — closing the PTY — only
 * when the tab itself is closed (removed from `tabs`).
 */
export function TabContent() {
  const { tabs, activeTabId } = useTabContext();
  const activeTab = tabs.find((t) => t.id === activeTabId);

  if (!activeTab) {
    return (
      <div className="empty-state" style={{ margin: "auto", padding: "40px", textAlign: "center" }}>
        <p style={{ color: "var(--text-muted)" }}>Open an organization from the ☰ menu to get started.</p>
      </div>
    );
  }

  const editorTabs = tabs.filter((t) => t.type === "editor");

  return (
    <>
      {editorTabs.map((t) => (
        <div key={t.id} style={{ display: t.id === activeTabId ? "flex" : "none", height: "100%", minHeight: 0 }}>
          {/* Hidden tabs stay mounted (PTY/xterm survive), but their hotkey
              handlers must not fire — see TabVisibleContext. */}
          <TabVisibleContext.Provider value={t.id === activeTabId}>
            <EditorWorkspace contextId={t.contextId!} contextKey={t.contextKey} initialCwd={t.initialCwd} />
          </TabVisibleContext.Provider>
        </div>
      ))}
      {activeTab.type !== "editor" && <NonEditorContent tab={activeTab} />}
    </>
  );
}
