import { useState, useRef, useEffect, useCallback, type FormEvent } from "react";
import { HashRouter, Routes, Route } from "react-router-dom";
import ContextList from "./pages/ContextList";
import ContextDetail from "./pages/ContextDetail";
import Settings from "./pages/Settings";
import HamburgerMenu from "./components/HamburgerMenu";
import { SetSlackPassphrase } from "../wailsjs/go/main/App";
import { ProgressProvider } from "./contexts/ProgressContext";
import ProgressModal from "./components/ProgressModal";
import { QuickAttachOverlay } from "./components/QuickAttachOverlay";
import { AccessibilityBanner } from "./components/AccessibilityBanner";
import { EventsOn, EventsOff, SendNotification, InitializeNotifications } from "../wailsjs/runtime/runtime";
import "./style.css";

function PassphraseGate({ onUnlocked }: { onUnlocked: () => void }) {
  const [value, setValue] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!value.trim()) {
      setError("Passphrase is required.");
      inputRef.current?.focus();
      return;
    }
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
          <input
            ref={inputRef}
            type="password"
            className="passphrase-input"
            placeholder="Enter passphrase…"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            autoFocus
            autoComplete="off"
          />
          {error && <div className="field-error passphrase-error">{error}</div>}
          <button type="submit" className="btn-primary passphrase-btn" disabled={submitting}>
            {submitting ? "Unlocking…" : "Continue"}
          </button>
        </form>
      </div>
    </div>
  );
}

const isWailsContext = () => typeof (window as unknown as Record<string, unknown>)["go"] !== "undefined";

/** Unsubscribes a Wails event listener. Handles both return-value styles across Wails versions. */
const safeOff = (off: unknown, event: string) => {
  if (typeof off === "function") (off as () => void)();
  else EventsOff(event);
};

interface PickerState {
  visible: boolean;
  text: string;
}

function App() {
  const [unlocked, setUnlocked] = useState(!isWailsContext());
  const [picker, setPicker] = useState<PickerState>({ visible: false, text: "" });
  const [showA11yBanner, setShowA11yBanner] = useState(false);
  const [toastMsg, setToastMsg] = useState("");
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const showToast = useCallback((msg: string) => {
    setToastMsg(msg);
    if (toastTimer.current) clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => setToastMsg(""), 3000);
  }, []);

  // Clear the toast timer on unmount.
  useEffect(() => {
    return () => {
      if (toastTimer.current) clearTimeout(toastTimer.current);
    };
  }, []);

  // Initialise notifications and listen for the global hotkey event.
  useEffect(() => {
    if (!isWailsContext()) return;
    InitializeNotifications().catch(() => {});

    const offPicker = EventsOn("hotkey:show-picker", (text: string) => {
      setPicker({ visible: true, text: text ?? "" });
    });
    const offA11y = EventsOn("accessibility:needed", () => {
      setShowA11yBanner(true);
    });
    const offA11yGranted = EventsOn("accessibility:granted", () => {
      setShowA11yBanner(false);
    });
    const offNoSelection = EventsOn("hotkey:no-selection", () => {
      showToast("No text selected — select text in another app first");
    });
    return () => {
      safeOff(offPicker, "hotkey:show-picker");
      safeOff(offA11y, "accessibility:needed");
      safeOff(offA11yGranted, "accessibility:granted");
      safeOff(offNoSelection, "hotkey:no-selection");
    };
  }, [showToast]);

  const handleAttach = useCallback(
    (contextKey: string) => {
      setPicker({ visible: false, text: "" });
      showToast(`Text attached to "${contextKey}"`);
      SendNotification({
        id: `quick-attach-${Date.now()}`,
        title: "Text attached",
        body: `Added to context: ${contextKey}`,
      }).catch(() => {});
    },
    [showToast]
  );

  const handleDismiss = useCallback(() => {
    setPicker({ visible: false, text: "" });
  }, []);

  if (!unlocked) {
    return <PassphraseGate onUnlocked={() => setUnlocked(true)} />;
  }

  return (
    <ProgressProvider>
      <ProgressModal />
      <HashRouter>
        <HamburgerMenu />
        <Routes>
          <Route path="/" element={<ContextList />} />
          <Route path="/context/:id" element={<ContextDetail />} />
          <Route path="/settings" element={<Settings />} />
        </Routes>
      </HashRouter>

      {showA11yBanner && (
        <AccessibilityBanner onDismiss={() => setShowA11yBanner(false)} />
      )}

      <QuickAttachOverlay
        visible={picker.visible}
        text={picker.text}
        onAttach={handleAttach}
        onDismiss={handleDismiss}
      />

      {toastMsg && <div className="toast toast--success">{toastMsg}</div>}
    </ProgressProvider>
  );
}

export default App;
