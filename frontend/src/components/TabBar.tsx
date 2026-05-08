import { useMemo } from "react";
import { X, Pin } from "lucide-react";
import { useTabContext } from "../contexts/TabContext";
import type { Tab } from "../types";

const ORG_COLORS = [
  "#F5A040",  // amber
  "#7CD4E7",  // cyan
  "#82D9A0",  // green
  "#B49CF5",  // purple
  "#E07878",  // red
  "#F5C97C",  // yellow
  "#F57CA0",  // pink
  "#C9E882",  // lime
];

function useOrgColorMap(tabs: Tab[]): Map<string, string> {
  return useMemo(() => {
    const map = new Map<string, string>();
    let idx = 0;
    for (const t of tabs) {
      const orgId = t.type === "org" ? t.orgId : t.type === "context" ? t.orgId : undefined;
      if (orgId && !map.has(orgId)) {
        map.set(orgId, ORG_COLORS[idx % ORG_COLORS.length]);
        idx++;
      }
    }
    return map;
  }, [tabs]);
}

function TabItem({ tab, isActive, orgColor }: { tab: Tab; isActive: boolean; orgColor?: string }) {
  const { tabs, setActiveTab, closeTab, pinTab, unpinTab } = useTabContext();

  const displayTitle = useMemo(() => {
    if (tab.type !== "context") return tab.title;
    const orgName = tab.orgName
      ?? tabs.find((t) => t.type === "org" && t.orgId === tab.orgId)?.title;
    const key = tab.contextKey ?? tab.title;
    return orgName ? `${orgName} / ${key}` : key;
  }, [tab, tabs]);

  const isOrg = tab.type === "org";

  const style: React.CSSProperties = orgColor
    ? ({ "--tab-org-color": orgColor } as React.CSSProperties)
    : {};

  return (
    <div
      className={`tab-item${isActive ? " tab-item--active" : ""}${isOrg ? " tab-item--org" : ""}${tab.pinned ? " tab-item--pinned" : ""}${orgColor ? " tab-item--colored" : ""}`}
      style={style}
      onClick={() => setActiveTab(tab.id)}
      onContextMenu={(e) => {
        e.preventDefault();
        tab.pinned ? unpinTab(tab.id) : pinTab(tab.id);
      }}
      title={tab.pinned ? `${displayTitle} (pinned — right-click to unpin)` : `${displayTitle} (right-click to pin)`}
    >
      <span className={`tab-item__title${isOrg ? " tab-item__title--org" : ""}`}>{displayTitle}</span>
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
  const orgColorMap = useOrgColorMap(tabs);

  if (tabs.length === 0) return null;

  return (
    <div className="tab-bar">
      {tabs.map((tab) => {
        const orgId = tab.type === "org" ? tab.orgId : tab.type === "context" ? tab.orgId : undefined;
        const orgColor = orgId ? orgColorMap.get(orgId) : undefined;
        return (
          <TabItem key={tab.id} tab={tab} isActive={tab.id === activeTabId} orgColor={orgColor} />
        );
      })}
    </div>
  );
}
