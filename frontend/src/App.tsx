import { useState, useRef, useEffect, useCallback } from "react";
import { TabProvider, useTabContext } from "./contexts/TabContext";
import { PassphraseProvider } from "./contexts/PassphraseContext";
import { TabContent } from "./components/TabContent";
import { EditorFilesProvider } from "./contexts/EditorFilesContext";
import { EditorFileTabBar } from "./components/editor/EditorFileTabBar";
import HamburgerMenu from "./components/HamburgerMenu";
import TabBar from "./components/TabBar";
import { ChevronLeft, ChevronRight, Sparkles } from "lucide-react";
import { ResizeHandle } from "./components/editor/ResizeHandle";
import { nextPaneSize, readStoredPaneSize } from "./lib/paneSize";
import { ListAllContexts, ListOrganizations, GetKeybindings, AppVersion } from "../wailsjs/go/main/App";
import { ProgressProvider } from "./contexts/ProgressContext";
import ProgressModal from "./components/ProgressModal";
import { QuickAttachOverlay } from "./components/QuickAttachOverlay";
import { AccessibilityBanner } from "./components/AccessibilityBanner";
import { BgRefreshToast } from "./components/BgRefreshToast";
import { EventsOn, EventsOff, SendNotification, InitializeNotifications, Environment } from "../wailsjs/runtime/runtime";
import { hookNotificationOptions, type HookNotifyPayload } from "./lib/hookNotify";
import { HotkeyProvider } from "./hotkeys/HotkeyProvider";
import { HotkeyHelpModal } from "./components/HotkeyHelpModal";
import { GlobalSearchModal } from "./components/GlobalSearchModal";
import { GlobalHotkeys } from "./components/GlobalHotkeys";
import { EditorOpenPicker } from "./components/EditorOpenPicker";
import type { Platform } from "./hotkeys/chord";
import type { Keybinding } from "./types";
import logo from "./assets/images/logo.png";
import "./style.css";

// The context rail starts as an icon-only strip; dragging it wide enough gives
// context names room to render in full instead of truncating to "juna-healt…".
export const SIDEBAR_RAIL_KEY = "vibecodeway:app:sidebarRailWidth";
export const RAIL_MIN = 72;   // the icon-only width the rail shipped with
export const RAIL_MAX = 320;
const RAIL_EXPANDED = 200; // what the toggle opens to before the user drags their own width

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
  const [railWidth, setRailWidth] = useState(() =>
    readStoredPaneSize(localStorage.getItem(SIDEBAR_RAIL_KEY), RAIL_MIN, RAIL_MIN, RAIL_MAX)
  );
  useEffect(() => { localStorage.setItem(SIDEBAR_RAIL_KEY, String(railWidth)); }, [railWidth]);
  // ponytail: remembered in a ref, not persisted — a restart re-opens at
  // RAIL_EXPANDED. Give it its own storage key if that starts to grate.
  const lastExpanded = useRef(RAIL_EXPANDED);
  const toggleRail = () =>
    setRailWidth((w) => {
      if (w > RAIL_MIN) { lastExpanded.current = w; return RAIL_MIN; }
      return lastExpanded.current;
    });
  const [showA11yBanner, setShowA11yBanner] = useState(false);
  const [toastMsg, setToastMsg] = useState("");
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { openContextTab, openAskTab, tabs, activeTabId, forceCloseTab } = useTabContext();
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
        <div
          className={`app-sidebar${railWidth >= 120 ? " app-sidebar--wide" : ""}`}
          // --rail-w drives .tab-item__title's max-width so context names get
          // wider as the rail does, instead of always ellipsising at 62px.
          style={{ width: railWidth, "--rail-w": `${railWidth}px` } as React.CSSProperties}
        >
          <HamburgerMenu />
          <img className="app-logo" src={logo} alt="VibeCodeWay" title="VibeCodeWay" />
          <button className="app-ask-ai" onClick={openAskTab} title="Ask AI" aria-label="Ask AI">
            <Sparkles size={14} />
            <span className="app-ask-ai__label">Ask AI</span>
          </button>
          <TabBar />
          <button
            className="app-sidebar__toggle"
            onClick={toggleRail}
            aria-label={railWidth > RAIL_MIN ? "Collapse sidebar" : "Expand sidebar"}
            title={railWidth > RAIL_MIN ? "Collapse sidebar" : "Expand sidebar"}
          >
            {railWidth > RAIL_MIN ? <ChevronLeft size={14} /> : <ChevronRight size={14} />}
          </button>
          <AppVersionBadge />
        </div>
        <ResizeHandle
          orientation="vertical"
          ariaLabel="resize sidebar rail"
          onResize={(dx) => setRailWidth((w) => nextPaneSize(w, dx, RAIL_MIN, RAIL_MAX))}
        />
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

// Build version pinned to the sidebar bottom. Stamped via -ldflags at build
// time; "dev" (unstamped) builds show nothing — the badge only means something
// for installed versions.
function AppVersionBadge() {
  const [version, setVersion] = useState("");
  useEffect(() => {
    AppVersion().then(setVersion).catch(() => {});
  }, []);
  if (!version || version === "dev") return null;
  return (
    <div title={`VibeCodeWay ${version}`}
      style={{ fontSize: 9, color: "var(--text-muted)", opacity: 0.7, textAlign: "center", padding: "2px 4px 6px", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
      {version}
    </div>
  );
}

// ─── Root ─────────────────────────────────────────────────────────────────────

function App() {
  const [platform, setPlatform] = useState<Platform>("other");
  const [bindings, setBindings] = useState<Keybinding[]>([]);

  useEffect(() => {
    Environment().then((i) => setPlatform(i.platform === "darwin" ? "darwin" : "other")).catch(() => {});
  }, []);

  useEffect(() => {
    const load = () => GetKeybindings().then((b) => setBindings(b ?? [])).catch(() => {});
    void load();
    // Re-fetch when a rebind is saved anywhere (help modal, Settings) so the
    // live keymap reflects the new combos without an app restart.
    window.addEventListener("keybindings-changed", load);
    return () => window.removeEventListener("keybindings-changed", load);
  }, []);

  return (
    <HotkeyProvider platform={platform} keybindings={bindings}>
      <PassphraseProvider>
        <TabProvider>
          <EditorFilesProvider>
            <AppShell />
          </EditorFilesProvider>
          <GlobalHotkeys />
          <GlobalSearchModal />
          <EditorOpenPicker />
        </TabProvider>
      </PassphraseProvider>
      <HotkeyHelpModal />
    </HotkeyProvider>
  );
}

export default App;
