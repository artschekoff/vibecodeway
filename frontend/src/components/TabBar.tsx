import { useMemo } from "react";
import { X, Pin, Globe, FileCode2, Settings, ScrollText, Sparkles } from "lucide-react";
import {
  DndContext,
  closestCenter,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  verticalListSortingStrategy,
  useSortable,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { useTabContext } from "../contexts/TabContext";
import type { Tab } from "../types";

const ORG_COLORS = [
  "#F5A040",
  "#7CD4E7",
  "#82D9A0",
  "#B49CF5",
  "#E07878",
  "#F5C97C",
  "#F57CA0",
  "#C9E882",
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

function TabIcon({ tab, orgColor }: { tab: Tab; orgColor?: string }) {
  if (tab.type === "org") {
    const letter = (tab.title ?? "?")[0].toUpperCase();
    return (
      <span
        className="tab-item__org-dot"
        style={{ background: orgColor ?? "var(--surface2)" }}
      >
        {letter}
      </span>
    );
  }
  if (tab.type === "context") return <FileCode2 size={20} />;
  if (tab.type === "settings") return <Settings size={20} />;
  if (tab.type === "logs") return <ScrollText size={20} />;
  if (tab.type === "prompts") return <Sparkles size={20} />;
  return <Globe size={20} />;
}

function shortLabel(tab: Tab): string {
  if (tab.type === "settings") return "Settings";
  if (tab.type === "logs") return "Logs";
  if (tab.type === "prompts") return "Prompts";
  return tab.contextKey ?? tab.title ?? "…";
}

function TabItem({ tab, isActive, orgColor }: { tab: Tab; isActive: boolean; orgColor?: string }) {
  const { setActiveTab, closeTab, pinTab, unpinTab } = useTabContext();
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: tab.id });

  const isOrg = tab.type === "org";

  const dragStyle: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.45 : 1,
    zIndex: isDragging ? 10 : undefined,
    cursor: isDragging ? "grabbing" : "grab",
  };

  const label = shortLabel(tab);

  return (
    <div
      ref={setNodeRef}
      className={`tab-item${isActive ? " tab-item--active" : ""}${isOrg ? " tab-item--org" : ""}${tab.pinned ? " tab-item--pinned" : ""}${orgColor ? " tab-item--colored" : ""}`}
      style={dragStyle}
      onClick={() => setActiveTab(tab.id)}
      onContextMenu={(e) => {
        e.preventDefault();
        tab.pinned ? unpinTab(tab.id) : pinTab(tab.id);
      }}
      title={tab.pinned ? `${label} (pinned)` : label}
      {...attributes}
      {...listeners}
    >
      <div className="tab-item__icon">
        <TabIcon tab={tab} orgColor={orgColor} />
      </div>
      <span className={`tab-item__title${isOrg ? " tab-item__title--org" : ""}`}>
        {label}
      </span>
      <span className="tab-item__close-zone">
        {tab.pinned ? (
          <button
            className="tab-item__action"
            onPointerDown={(e) => e.stopPropagation()}
            onClick={(e) => { e.stopPropagation(); unpinTab(tab.id); }}
            title="Unpin"
          >
            <Pin size={9} />
          </button>
        ) : (
          <button
            className="tab-item__action"
            onPointerDown={(e) => e.stopPropagation()}
            onClick={(e) => { e.stopPropagation(); closeTab(tab.id); }}
            title="Close"
          >
            <X size={9} />
          </button>
        )}
      </span>
    </div>
  );
}

export default function TabBar() {
  const { tabs, activeTabId, reorderTabs } = useTabContext();
  const orgColorMap = useOrgColorMap(tabs);
  const tabIds = useMemo(() => tabs.map((t) => t.id), [tabs]);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } })
  );

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (over && active.id !== over.id) {
      reorderTabs(active.id as string, over.id as string);
    }
  };

  if (tabs.length === 0) return null;

  return (
    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
      <SortableContext items={tabIds} strategy={verticalListSortingStrategy}>
        <div className="tab-bar">
          {tabs.map((tab) => {
            const orgId = tab.type === "org" ? tab.orgId : tab.type === "context" ? tab.orgId : undefined;
            const orgColor = orgId ? orgColorMap.get(orgId) : undefined;
            return (
              <TabItem key={tab.id} tab={tab} isActive={tab.id === activeTabId} orgColor={orgColor} />
            );
          })}
        </div>
      </SortableContext>
    </DndContext>
  );
}
