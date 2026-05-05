import { useEffect, useRef, useState } from "react";
import { ArrowLeft, Copy, Check } from "lucide-react";
import { GetAISettings, SetAISettings, GetSupportedPlatforms, GetVCSAuth, VCSLoginStart, VCSLoginComplete, GetAppSettings, SetAppSettings } from "../../wailsjs/go/main/App";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import type { AISettings, AppSettings, VCSPlatformInfo, VCSPlatform, DeviceCodeInfo } from "../types";
import { useTabContext } from "../contexts/TabContext";

export default function SettingsPage() {
  const { activeTabId, closeTab } = useTabContext();
  const hotkeysRef = useRef<HTMLElement>(null);
  const [settings, setSettings] = useState<AISettings | null>(null);
  const [apiKey, setApiKey] = useState("");
  const [model, setModel] = useState("");
  const [baseUrl, setBaseUrl] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const [platforms, setPlatforms] = useState<VCSPlatformInfo[]>([]);
  const [vcsAuth, setVcsAuth] = useState<Record<VCSPlatform, string>>({} as Record<VCSPlatform, string>);
  const [vcsConnecting, setVcsConnecting] = useState<VCSPlatform | null>(null);
  const [vcsMessage, setVcsMessage] = useState("");
  const [deviceCode, setDeviceCode] = useState<DeviceCodeInfo | null>(null);
  const [codeCopied, setCodeCopied] = useState(false);

  // Background refresh settings
  const [appSettings, setAppSettings] = useState<AppSettings | null>(null);
  const [contextRefetchEnabled, setContextRefetchEnabled] = useState(false);
  const [contextRefetchInterval, setContextRefetchInterval] = useState(30);
  const [reviewsRefetchEnabled, setReviewsRefetchEnabled] = useState(false);
  const [reviewsRefetchInterval, setReviewsRefetchInterval] = useState(30);
  const [terminalAppPath, setTerminalAppPath] = useState("");
  const [savingRefetch, setSavingRefetch] = useState(false);
  const [refetchMessage, setRefetchMessage] = useState("");

  useEffect(() => {
    let cancelled = false;
    Promise.all([
      GetAISettings(),
      GetSupportedPlatforms().catch(() => []),
      GetAppSettings().catch(() => null),
    ])
      .then(([s, plats, appS]) => {
        if (cancelled) return;
        const typed = s as unknown as AISettings;
        setSettings(typed);
        setModel(typed.model);
        setBaseUrl(typed.baseUrl);

        if (appS) {
          const typedAppS = appS as unknown as AppSettings;
          setAppSettings(typedAppS);
          setContextRefetchEnabled(typedAppS.contextRefetchEnabled);
          setContextRefetchInterval(typedAppS.contextRefetchIntervalMin);
          setReviewsRefetchEnabled(typedAppS.reviewsRefetchEnabled);
          setReviewsRefetchInterval(typedAppS.reviewsRefetchIntervalMin);
          setTerminalAppPath(typedAppS.terminalAppPath ?? "");
        }

        const typedPlats = (plats ?? []) as VCSPlatformInfo[];
        setPlatforms(typedPlats);

        // Load auth status for each platform
        return Promise.all(
          typedPlats.map((p) =>
            GetVCSAuth(p.platform)
              .then((u) => [p.platform, (u as any)?.username ?? ""] as [VCSPlatform, string])
              .catch(() => [p.platform, ""] as [VCSPlatform, string])
          )
        );
      })
      .then((auths) => {
        if (cancelled || !auths) return;
        const map: Record<VCSPlatform, string> = {} as Record<VCSPlatform, string>;
        for (const [platform, username] of auths) map[platform] = username;
        setVcsAuth(map);
      })
      .catch((e) => {
        if (cancelled) return;
        setError(String(e));
      })
      .finally(() => {
        if (cancelled) return;
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const handleVCSConnect = async (platform: VCSPlatform) => {
    setVcsConnecting(platform);
    setVcsMessage("");
    setDeviceCode(null);
    try {
      const info = await VCSLoginStart(platform) as unknown as DeviceCodeInfo;
      setDeviceCode(info);
      BrowserOpenURL(info.verificationUri);

      const username = await VCSLoginComplete(platform, info.deviceCode, info.interval) as string;
      setDeviceCode(null);
      setVcsAuth((prev) => ({ ...prev, [platform]: username }));
      setVcsMessage(`Connected as ${username}`);
    } catch (e) {
      setDeviceCode(null);
      setVcsMessage(String(e));
    } finally {
      setVcsConnecting(null);
    }
  };

  const handleSaveRefetch = async () => {
    setSavingRefetch(true);
    setRefetchMessage("");
    try {
      // SetAppSettings binding accepts 5th arg (terminalAppPath) once regenerated
      await (SetAppSettings as (a: boolean, b: number, c: boolean, d: number, e: string) => Promise<void>)(
        contextRefetchEnabled,
        Math.max(1, contextRefetchInterval),
        reviewsRefetchEnabled,
        Math.max(1, reviewsRefetchInterval),
        terminalAppPath.trim(),
      );
      const updated = await GetAppSettings() as unknown as AppSettings;
      setAppSettings(updated);
      setTerminalAppPath(updated.terminalAppPath ?? "");
      setRefetchMessage("Saved.");
    } catch (e) {
      setError(String(e));
    } finally {
      setSavingRefetch(false);
    }
  };

  const handleSave = async () => {
    setSaving(true);
    setError("");
    setMessage("");
    try {
      await SetAISettings(apiKey, model, baseUrl);
      const updated = await GetAISettings() as unknown as AISettings;
      setSettings(updated);
      setModel(updated.model);
      setBaseUrl(updated.baseUrl);
      setApiKey("");
      setMessage("Settings saved.");
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <div className="loading">Loading…</div>;

  const sourceLabel =
    settings?.source === "env"
      ? "Environment variable OPENAI_API_KEY"
      : settings?.source === "db"
      ? "Stored in app (encrypted)"
      : "Not configured";

  return (
    <div className="page">
      <div className="topbar">
        <button className="back-btn" onClick={() => activeTabId && closeTab(activeTabId)}><ArrowLeft size={16} /> Back</button>
        <div style={{ flex: 1 }}>
          <h1 style={{ fontSize: 16, margin: 0 }}>Settings · OpenAI</h1>
        </div>
      </div>

      <div className="content" style={{ maxWidth: 620 }}>
        {error && <div className="error-banner">{error}</div>}
        {message && <div className="toast toast--success">{message}</div>}

        <section className="detail-section" id="hotkeys" ref={hotkeysRef}>
          <div className="detail-section-header">
            <span className="detail-section-title">Keyboard Shortcuts</span>
          </div>
          <div style={{ padding: "12px 14px", display: "flex", flexDirection: "column", gap: 12 }}>
            <div className="hotkey-row">
              <div className="hotkey-keys">
                <kbd className="hotkey-kbd">⌘</kbd>
                <kbd className="hotkey-kbd">⇧</kbd>
                <kbd className="hotkey-kbd">C</kbd>
              </div>
              <div className="hotkey-desc">
                <div style={{ fontSize: 13, fontWeight: 500 }}>Quick Attach</div>
                <div style={{ fontSize: 12, color: "var(--text-muted)", marginTop: 2 }}>
                  Выдели текст в любом приложении и нажми это сочетание — откроется выбор контекста,
                  к которому прикрепить текст. Работает глобально, даже когда приложение свёрнуто.
                </div>
              </div>
            </div>
            <div className="hotkey-row">
              <div className="hotkey-keys">
                <kbd className="hotkey-kbd">⌘</kbd>
                <kbd className="hotkey-kbd">↵</kbd>
              </div>
              <div className="hotkey-desc">
                <div style={{ fontSize: 13, fontWeight: 500 }}>Отправить сообщение в чате</div>
                <div style={{ fontSize: 12, color: "var(--text-muted)", marginTop: 2 }}>
                  В окне чата контекста — отправляет сообщение без перехода на новую строку.
                </div>
              </div>
            </div>
          </div>
        </section>

        {platforms.length > 0 && (
          <section className="detail-section">
            <div className="detail-section-header">
              <span className="detail-section-title">Connected Accounts</span>
            </div>
            {deviceCode && (
              <div className="device-code-banner">
                <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 6 }}>
                  Enter this code in the browser to authorize:
                </div>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 8 }}>
                  <div className="device-code-value">{deviceCode.userCode}</div>
                  <button
                    className="btn-icon"
                    title="Copy code"
                    onClick={() => {
                      void navigator.clipboard.writeText(deviceCode.userCode).then(() => {
                        setCodeCopied(true);
                        setTimeout(() => setCodeCopied(false), 2000);
                      });
                    }}
                  >
                    {codeCopied ? <Check size={15} color="var(--green)" /> : <Copy size={15} />}
                  </button>
                </div>
                <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 6 }}>
                  Waiting for authorization…
                </div>
              </div>
            )}
            {vcsMessage && !deviceCode && (
              <div className="toast toast--success" style={{ margin: "8px 14px", position: "static" }}>{vcsMessage}</div>
            )}
            <div style={{ display: "flex", flexDirection: "column", gap: 0 }}>
              {platforms.map((p) => {
                const connected = !!vcsAuth[p.platform];
                return (
                  <div key={p.platform} className="vcs-auth-row">
                    <span className="platform-badge">{p.platform}</span>
                    <span className="vcs-auth-label">{p.label}</span>
                    <span className="vcs-auth-status">
                      {connected ? (
                        <span className="vcs-auth-connected">✓ {vcsAuth[p.platform]}</span>
                      ) : (
                        <span className="vcs-auth-disconnected">Not connected</span>
                      )}
                    </span>
                    {p.enabled ? (
                      <button
                        className="btn-secondary"
                        style={{ fontSize: 12, padding: "3px 10px" }}
                        disabled={vcsConnecting === p.platform}
                        onClick={() => void handleVCSConnect(p.platform)}
                      >
                        {vcsConnecting === p.platform ? "Connecting…" : connected ? "Reconnect" : "Connect"}
                      </button>
                    ) : (
                      <span className="platform-disabled">Coming soon</span>
                    )}
                  </div>
                );
              })}
            </div>
          </section>
        )}

        <section className="detail-section">
          <div className="detail-section-header">
            <span className="detail-section-title">Background Refresh</span>
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 16, padding: "12px 14px" }}>
            {refetchMessage && <div className="toast toast--success" style={{ position: "static", margin: 0 }}>{refetchMessage}</div>}

            {/* Contexts */}
            <div>
              <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13, cursor: "pointer" }}>
                <input
                  type="checkbox"
                  checked={contextRefetchEnabled}
                  onChange={(e) => setContextRefetchEnabled(e.target.checked)}
                />
                Enable background refetch for Contexts
              </label>
              <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 8 }}>
                <label style={{ fontSize: 12, color: "var(--text-muted)", whiteSpace: "nowrap" }}>
                  Interval (minutes)
                </label>
                <input
                  type="number"
                  className="passphrase-input"
                  min={1}
                  max={1440}
                  value={contextRefetchInterval}
                  disabled={!contextRefetchEnabled}
                  onChange={(e) => setContextRefetchInterval(Math.max(1, Number(e.target.value)))}
                  style={{ width: 80 }}
                />
              </div>
            </div>

            {/* Code Reviews */}
            <div>
              <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13, cursor: "pointer" }}>
                <input
                  type="checkbox"
                  checked={reviewsRefetchEnabled}
                  onChange={(e) => setReviewsRefetchEnabled(e.target.checked)}
                />
                Enable background refetch for Code Reviews
              </label>
              <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 8 }}>
                <label style={{ fontSize: 12, color: "var(--text-muted)", whiteSpace: "nowrap" }}>
                  Interval (minutes)
                </label>
                <input
                  type="number"
                  className="passphrase-input"
                  min={1}
                  max={1440}
                  value={reviewsRefetchInterval}
                  disabled={!reviewsRefetchEnabled}
                  onChange={(e) => setReviewsRefetchInterval(Math.max(1, Number(e.target.value)))}
                  style={{ width: 80 }}
                />
              </div>
            </div>

            <div>
              <button
                className="btn-primary"
                onClick={() => void handleSaveRefetch()}
                disabled={savingRefetch}
              >
                {savingRefetch ? "Saving…" : "Save"}
              </button>
            </div>

            <div style={{ fontSize: 11, color: "var(--text-muted)" }}>
              When enabled, the app will silently refresh data in the background at the specified interval.
              Changes take effect immediately without restarting the app.
            </div>
          </div>
        </section>

        <section className="detail-section">
          <div className="detail-section-header">
            <span className="detail-section-title">Terminal</span>
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 12, padding: "12px 14px" }}>
            <div>
              <label style={{ display: "block", fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>
                Terminal app name or .app path
              </label>
              <input
                className="passphrase-input"
                placeholder="kitty  or  /Applications/iTerm.app"
                value={terminalAppPath}
                onChange={(e) => setTerminalAppPath(e.target.value)}
                style={{ width: "100%" }}
              />
              <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 4 }}>
                Used when checking out a branch. Enter an app name (e.g. <code>kitty</code>,{" "}
                <code>iTerm</code>, <code>Terminal</code>) or full .app path. Saved with the Background Refresh Save button above.
              </div>
            </div>
          </div>
        </section>

        <section className="detail-section">
          <div className="detail-section-header">
            <span className="detail-section-title">OpenAI credentials</span>
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: 12, padding: "12px 14px" }}>
            <div>
              <label style={{ display: "block", fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>
                Current source
              </label>
              <div style={{ fontSize: 13 }}>
                <strong>{sourceLabel}</strong>
              </div>
            </div>

            <div>
              <label style={{ display: "block", fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>
                API key {settings?.hasKey ? "(leave empty to keep existing)" : ""}
              </label>
              <input
                type="password"
                className="passphrase-input"
                placeholder={settings?.hasKey ? "•••••••••••••••" : "sk-…"}
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                autoComplete="off"
                style={{ width: "100%" }}
              />
              {settings?.source === "env" && (
                <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 4 }}>
                  The environment variable <code>OPENAI_API_KEY</code> is currently active and
                  overrides any key stored here.
                </div>
              )}
            </div>

            <div>
              <label style={{ display: "block", fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>
                Model
              </label>
              <input
                className="passphrase-input"
                placeholder="gpt-4o-mini"
                value={model}
                onChange={(e) => setModel(e.target.value)}
                style={{ width: "100%" }}
              />
            </div>

            <div>
              <label style={{ display: "block", fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>
                Base URL (optional, e.g. self-hosted compatible endpoint)
              </label>
              <input
                className="passphrase-input"
                placeholder="https://api.openai.com"
                value={baseUrl}
                onChange={(e) => setBaseUrl(e.target.value)}
                style={{ width: "100%" }}
              />
            </div>

            <div>
              <button
                className="btn-primary"
                onClick={() => void handleSave()}
                disabled={saving}
              >
                {saving ? "Saving…" : "Save"}
              </button>
            </div>

            <div style={{ fontSize: 11, color: "var(--text-muted)" }}>
              API keys are encrypted with your Slack passphrase before being stored on disk. You must
              unlock the app with that passphrase before saving a key for the first time.
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}
