import { useState, useRef, type FormEvent } from "react";
import { HashRouter, Routes, Route } from "react-router-dom";
import ContextList from "./pages/ContextList";
import ContextDetail from "./pages/ContextDetail";
import Settings from "./pages/Settings";
import { SetSlackPassphrase } from "../wailsjs/go/main/App";
import { ProgressProvider } from "./contexts/ProgressContext";
import ProgressModal from "./components/ProgressModal";
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

function App() {
  const [unlocked, setUnlocked] = useState(!isWailsContext());

  if (!unlocked) {
    return <PassphraseGate onUnlocked={() => setUnlocked(true)} />;
  }

  return (
    <ProgressProvider>
      <ProgressModal />
      <HashRouter>
        <Routes>
          <Route path="/" element={<ContextList />} />
          <Route path="/context/:id" element={<ContextDetail />} />
          <Route path="/settings" element={<Settings />} />
        </Routes>
      </HashRouter>
    </ProgressProvider>
  );
}

export default App;
