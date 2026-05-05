import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";
import { HasSlackPassphrase, SetSlackPassphrase } from "../../wailsjs/go/main/App";

interface PassphraseContextValue {
  isSet: boolean;
  ensurePassphrase(): Promise<boolean>;
}

const PassphraseContext = createContext<PassphraseContextValue | null>(null);

type Resolver = (value: boolean) => void;

const isWailsCtx = () =>
  typeof (window as unknown as Record<string, unknown>)["go"] !== "undefined";

export function PassphraseProvider({ children }: { children: ReactNode }) {
  const [isSet, setIsSet] = useState(false);
  const [showModal, setShowModal] = useState(false);
  const [value, setValue] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const pending = useRef<Resolver[]>([]);

  // Initialise: if passphrase was provided via env var, mark as already set.
  useEffect(() => {
    if (!isWailsCtx()) return;
    HasSlackPassphrase()
      .then((set) => setIsSet(set))
      .catch(() => {});
  }, []);

  const resolveAll = useCallback((result: boolean) => {
    const resolvers = pending.current;
    pending.current = [];
    resolvers.forEach((r) => r(result));
  }, []);

  const ensurePassphrase = useCallback((): Promise<boolean> => {
    if (!isWailsCtx() || isSet) return Promise.resolve(true);
    return new Promise<boolean>((resolve) => {
      pending.current.push(resolve);
      setShowModal(true);
    });
  }, [isSet]);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!value.trim()) {
      setError("Passphrase is required.");
      return;
    }
    setSubmitting(true);
    setError("");
    try {
      await SetSlackPassphrase(value.trim());
      setIsSet(true);
      setShowModal(false);
      setValue("");
      resolveAll(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to set passphrase.");
      setSubmitting(false);
    }
  };

  const handleDismiss = () => {
    setShowModal(false);
    setValue("");
    setError("");
    resolveAll(false);
  };

  return (
    <PassphraseContext.Provider value={{ isSet, ensurePassphrase }}>
      {children}
      {showModal && (
        <div className="confirm-overlay" style={{ zIndex: 300 }}>
          <div className="passphrase-card">
            <div className="passphrase-icon">🔑</div>
            <h1 className="passphrase-title">Slack Passphrase</h1>
            <p className="passphrase-desc">
              Enter your <code>SLACK_MCP_PASSPHRASE</code> to authenticate Slack CLI calls.
            </p>
            <form onSubmit={(e) => void handleSubmit(e)} className="passphrase-form">
              <input
                type="password"
                className="passphrase-input"
                placeholder="Enter passphrase…"
                value={value}
                onChange={(e) => setValue(e.target.value)}
                autoFocus
                autoComplete="off"
              />
              {error && <div className="field-error passphrase-error">{error}</div>}
              <button
                type="submit"
                className="btn-primary passphrase-btn"
                disabled={submitting}
              >
                {submitting ? "Unlocking…" : "Continue"}
              </button>
              <button
                type="button"
                className="btn-secondary passphrase-btn"
                style={{ marginTop: -4 }}
                onClick={handleDismiss}
              >
                Cancel
              </button>
            </form>
          </div>
        </div>
      )}
    </PassphraseContext.Provider>
  );
}

export function usePassphrase(): PassphraseContextValue {
  const ctx = useContext(PassphraseContext);
  if (!ctx) throw new Error("usePassphrase must be used within PassphraseProvider");
  return ctx;
}
