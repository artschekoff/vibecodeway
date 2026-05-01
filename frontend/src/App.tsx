import { useState, useRef, useEffect, useCallback, type FormEvent } from "react";
import { TabProvider, useTabContext } from "./contexts/TabContext";
import ContextList from "./pages/ContextList";
import ContextDetail from "./pages/ContextDetail";
import Settings from "./pages/Settings";
import HamburgerMenu from "./components/HamburgerMenu";
import TabBar from "./components/TabBar";
import { SetSlackPassphrase } from "../wailsjs/go/main/App";
import { ProgressProvider } from "./contexts/ProgressContext";
import ProgressModal from "./components/ProgressModal";
import { QuickAttachOverlay } from "./components/QuickAttachOverlay";
import { AccessibilityBanner } from "./components/AccessibilityBanner";
import { EventsOn, EventsOff, SendNotification, InitializeNotifications } from "../wailsjs/runtime/runtime";
import "./style.css";

// ─── Passphrase gate ─────────────────────────────────────────────────────────

function PassphraseGate({ onUnlocked }: { onUnlocked: () => void }) {
  const [value, setValue] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!value.trim()) { setError("Passphrase is required."); inputRef.current?.focus(); return; }
    setSubmitting(true);
    setError("");
    try {
      await SetSlackPassphrase(value.trim());
      onUnlocked();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to set passphrase — Wails bridge not available.");
      setSubmitting(false);
    }
  };

  return (
    <div className="passphrase-screen">
      <div className="passphrase-card">
        <div className="passphrase-icon">🔑</div>
        <h1 className="passphrase-title">Slack Passphrase</h1>
        <p className="passphrase-desc">
          Enter your <code>SLACK_MCP_PASSPHRASE</code> to authenticate Slack CLI calls.
        </p>
        <form onSubmit={(e) => void handleSubmit(e)} className="passphrase-form">
          <input ref={inputRef} type="password" className="passphrase-input"
            placeholder="Enter passphrase…" value={value}
            onChange={(e) => setValue(e.target.value)} autoFocus autoComplete="off" />
          {error && <div className="field-error passphrase-error">{error}</div>}
          <button type="submit" className="btn-primary passphrase-btn" disabled={submitting}>
            {submitting ? "Unlocking…" : "Continue"}
          </button>
        </form>
      </div>
    </div>
  );
}

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
  const { openContextTab } = useTabContext();

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

  const handleAttach = useCallback(
    (contextKey: string, contextId: string) => {
      setPicker({ visible: false, text: "" });
      showToast(`Attached to "${contextKey}"`);
      window.dispatchEvent(new CustomEvent("context:updated", { detail: { id: contextId } }));
      openContextTab(contextId, contextKey);
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
        <div className="app-topbar">
          <HamburgerMenu />
        </div>
        <TabBar />
        <div className="app-content">
          <TabContent />
        </div>
      </div>
      {showA11yBanner && <AccessibilityBanner onDismiss={() => setShowA11yBanner(false)} />}
      <QuickAttachOverlay
        visible={picker.visible}
        text={picker.text}
        onAttach={handleAttach}
        onDismiss={() => setPicker({ visible: false, text: "" })}
      />
      {toastMsg && <div className="toast toast--success">{toastMsg}</div>}
    </ProgressProvider>
  );
}

// ─── Root ─────────────────────────────────────────────────────────────────────

function App() {
  const [unlocked, setUnlocked] = useState(!isWailsContext());
  if (!unlocked) return <PassphraseGate onUnlocked={() => setUnlocked(true)} />;
  return (
    <TabProvider>
      <AppShell />
    </TabProvider>
  );
}

export default App;
