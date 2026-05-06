import { useMemo } from "react";
import { X, Pin } from "lucide-react";
import { useTabContext } from "../contexts/TabContext";
import type { Tab } from "../types";

function TabItem({ tab, isActive }: { tab: Tab; isActive: boolean }) {
  const { tabs, setActiveTab, closeTab, pinTab, unpinTab } = useTabContext();

  const displayTitle = useMemo(() => {
    if (tab.type !== "context") return tab.title;
    const orgName = tab.orgName
      ?? tabs.find((t) => t.type === "org" && t.orgId === tab.orgId)?.title;
    const key = tab.contextKey ?? tab.title;
    return orgName ? `${orgName} / ${key}` : key;
  }, [tab, tabs]);

  return (
    <div
      className={`tab-item${isActive ? " tab-item--active" : ""}${tab.type === "org" ? " tab-item--org" : ""}${tab.pinned ? " tab-item--pinned" : ""}`}
      onClick={() => setActiveTab(tab.id)}
      onContextMenu={(e) => {
        e.preventDefault();
        tab.pinned ? unpinTab(tab.id) : pinTab(tab.id);
      }}
      title={tab.pinned ? `${displayTitle} (pinned — right-click to unpin)` : `${displayTitle} (right-click to pin)`}
    >
      <span className="tab-item__title">{displayTitle}</span>
      {tab.pinned ? (
        <button
          className="tab-item__action"
          onClick={(e) => { e.stopPropagation(); unpinTab(tab.id); }}
          title="Unpin tab"
        >
          <Pin size={11} />
        </button>
      ) : (
        <button
          className="tab-item__action"
          onClick={(e) => { e.stopPropagation(); closeTab(tab.id); }}
          title="Close tab"
        >
          <X size={11} />
        </button>
      )}
    </div>
  );
}

export default function TabBar() {
  const { tabs, activeTabId } = useTabContext();

  if (tabs.length === 0) return null;

  return (
    <div className="tab-bar">
      {tabs.map((tab) => (
        <TabItem key={tab.id} tab={tab} isActive={tab.id === activeTabId} />
      ))}
    </div>
  );
}
