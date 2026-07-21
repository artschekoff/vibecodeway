import { useEffect, useRef, useState } from "react";
import { ArrowLeft, Copy, Check } from "lucide-react";
import { GetAISettings, SetAISettings, GetSupportedPlatforms, GetVCSAuth, VCSLoginStart, VCSLoginComplete, GetAppSettings, SetAppSettings, GetKeybindings, UpdateKeybinding } from "../../wailsjs/go/main/App";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import type { AISettings, AppSettings, VCSPlatformInfo, VCSPlatform, DeviceCodeInfo, Keybinding } from "../types";
import { useTabContext } from "../contexts/TabContext";
import WorkflowStatusManager from "../components/WorkflowStatusManager";
import KeybindingRow from "../components/KeybindingRow";
import GlobalEventHooks from "../components/GlobalEventHooks";

const OPENAI_MODELS = [
  // GPT-5 series
  { id: "gpt-5.5",       label: "GPT-5.5 (flagship)" },
  { id: "gpt-5.5-pro",   label: "GPT-5.5 Pro" },
  { id: "gpt-5.4",       label: "GPT-5.4" },
  { id: "gpt-5.4-pro",   label: "GPT-5.4 Pro" },
  { id: "gpt-5.4-mini",  label: "GPT-5.4 mini" },
  { id: "gpt-5.4-nano",  label: "GPT-5.4 nano" },
  { id: "gpt-5",         label: "GPT-5" },
  { id: "gpt-5-mini",    label: "GPT-5 mini" },
  { id: "gpt-5-nano",    label: "GPT-5 nano" },
  // GPT-4.1 series
  { id: "gpt-4.1",       label: "GPT-4.1 (recommended non-reasoning)" },
  { id: "gpt-4.1-mini",  label: "GPT-4.1 mini" },
  // GPT-4o series
  { id: "gpt-4o",        label: "GPT-4o" },
  { id: "gpt-4o-mini",   label: "GPT-4o mini" },
  // Reasoning (o-series)
  { id: "o3",            label: "o3" },
  { id: "o3-pro",        label: "o3 Pro" },
] as const;

const CLAUDE_MODELS = [
  { id: "claude-opus-4-7",           label: "Claude Opus 4.7 (most capable)" },
  { id: "claude-sonnet-4-6",         label: "Claude Sonnet 4.6 (recommended)" },
  { id: "claude-haiku-4-5-20251001", label: "Claude Haiku 4.5 (fast)" },
  { id: "claude-opus-4-6",           label: "Claude Opus 4.6" },
  { id: "claude-sonnet-4-5-20250929", label: "Claude Sonnet 4.5" },
  { id: "claude-opus-4-5-20251101",  label: "Claude Opus 4.5" },
  { id: "claude-opus-4-1-20250805",  label: "Claude Opus 4.1" },
] as const;

export default function SettingsPage() {
  const { activeTabId, closeTab, navigateBack } = useTabContext();
  const hotkeysRef = useRef<HTMLElement>(null);
  const [settings, setSettings] = useState<AISettings | null>(null);
  const [provider, setProvider] = useState<"openai" | "anthropic" | "local">("openai");
  const [apiKey, setApiKey] = useState("");
  const [anthropicKey, setAnthropicKey] = useState("");
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
  const [keybindings, setKeybindings] = useState<Keybinding[]>([]);
  const [deviceCode, setDeviceCode] = useState<DeviceCodeInfo | null>(null);
  const [codeCopied, setCodeCopied] = useState(false);

  // Background refresh settings
  const [appSettings, setAppSettings] = useState<AppSettings | null>(null);
  const [contextRefetchEnabled, setContextRefetchEnabled] = useState(false);
  const [contextRefetchInterval, setContextRefetchInterval] = useState(30);
  const [reviewsRefetchEnabled, setReviewsRefetchEnabled] = useState(false);
  const [reviewsRefetchInterval, setReviewsRefetchInterval] = useState(30);
  const [terminalAppPath, setTerminalAppPath] = useState("");
  const [codingAgentApp, setCodingAgentApp] = useState("cursor");
  const [savingRefetch, setSavingRefetch] = useState(false);
  const [refetchMessage, setRefetchMessage] = useState("");
  const [savingTerminal, setSavingTerminal] = useState(false);
  const [terminalMessage, setTerminalMessage] = useState("");

  useEffect(() => {
    let cancelled = false;
    GetKeybindings().then((kbs) => {
      if (!cancelled) setKeybindings(kbs as unknown as Keybinding[]);
    }).catch(() => {});
    Promise.all([
      GetAISettings(),
      GetSupportedPlatforms().catch(() => []),
      GetAppSettings().catch(() => null),
    ])
      .then(([s, plats, appS]) => {
        if (cancelled) return;
        const typed = s as unknown as AISettings;
        setSettings(typed);
        setProvider(typed.provider ?? "openai");
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
          setCodingAgentApp(typedAppS.codingAgentApp ?? "cursor");
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
      await SetAppSettings(
        contextRefetchEnabled,
        Math.max(1, contextRefetchInterval),
        reviewsRefetchEnabled,
        Math.max(1, reviewsRefetchInterval),
        terminalAppPath.trim(),
        codingAgentApp.trim(),
      );
      const updated = await GetAppSettings() as unknown as AppSettings;
      setAppSettings(updated);
      setTerminalAppPath(updated.terminalAppPath ?? "");
      setCodingAgentApp(updated.codingAgentApp ?? "cursor");
      setRefetchMessage("Saved.");
    } catch (e) {
      setError(String(e));
    } finally {
      setSavingRefetch(false);
    }
  };

  const handleSaveTerminal = async () => {
    setSavingTerminal(true);
    setTerminalMessage("");
    try {
      await SetAppSettings(
        contextRefetchEnabled,
        Math.max(1, contextRefetchInterval),
        reviewsRefetchEnabled,
        Math.max(1, reviewsRefetchInterval),
        terminalAppPath.trim(),
        codingAgentApp.trim(),
      );
      const updated = await GetAppSettings() as unknown as AppSettings;
      setAppSettings(updated);
      setTerminalAppPath(updated.terminalAppPath ?? "");
      setCodingAgentApp(updated.codingAgentApp ?? "cursor");
      setTerminalMessage("Saved.");
    } catch (e) {
      setError(String(e));
    } finally {
      setSavingTerminal(false);
    }
  };

  const handleSave = async () => {
    setSaving(true);
    setError("");
    setMessage("");
    try {
      await SetAISettings(provider, apiKey, anthropicKey, model, baseUrl);
      const updated = await GetAISettings() as unknown as AISettings;
      setSettings(updated);
      setProvider(updated.provider ?? "openai");
      setModel(updated.model);
      setBaseUrl(updated.baseUrl);
      setApiKey("");
      setAnthropicKey("");
      setMessage("Settings saved.");
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <div className="loading">Loading…</div>;

  return (
    <div className="page">
      <div className="topbar">
        <button className="back-btn" onClick={() => activeTabId && navigateBack(activeTabId)}><ArrowLeft size={16} /> Back</button>
        <div style={{ flex: 1 }}>
          <h1 style={{ fontSize: 16, margin: 0 }}>Settings</h1>
        </div>
      </div>

      <div className="content" style={{ maxWidth: 620 }}>
        {error && <div className="error-banner">{error}</div>}
        {message && <div className="toast toast--success">{message}</div>}

        <section className="detail-section" id="hotkeys" ref={hotkeysRef}>
          <div className="detail-section-header">
            <span className="detail-section-title">Keyboard Shortcuts</span>
          </div>
          <table className="keybindings-table">
            <thead>
              <tr>
                <th>Action</th>
                <th>Shortcut</th>
              </tr>
            </thead>
            <tbody>
              {keybindings.map((kb) => (
                <KeybindingRow
                  key={kb.id}
                  binding={kb}
                  onSave={async (command, modifierKeys, keyCode) => {
                    try {
                      await UpdateKeybinding(command, modifierKeys, keyCode);
                      setKeybindings((prev) =>
                        prev.map((b) =>
                          b.command === command ? { ...b, modifierKeys, keyCode } : b
                        )
                      );
                    } catch (e) {
                      console.error("Failed to update keybinding", e);
                    }
                  }}
                />
              ))}
            </tbody>
          </table>
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

        <GlobalEventHooks />

        <section className="detail-section">
          <div className="detail-section-header">
            <span className="detail-section-title">Terminal</span>
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 12, padding: "12px 14px" }}>
            {terminalMessage && <div className="toast toast--success" style={{ position: "static", margin: 0 }}>{terminalMessage}</div>}
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
                <code>iTerm</code>, <code>Terminal</code>) or full .app path.
              </div>
            </div>
            <div>
              <label style={{ display: "block", fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>
                Coding agent executable
              </label>
              <input
                className="passphrase-input"
                placeholder="cursor"
                value={codingAgentApp}
                onChange={(e) => setCodingAgentApp(e.target.value)}
                style={{ width: "100%" }}
              />
              <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 4 }}>
                Opened at the repo path after checkout. CLI executable name (e.g. <code>cursor</code>,{" "}
                <code>code</code>, <code>windsurf</code>).
              </div>
            </div>
            <div>
              <button
                className="btn-primary"
                onClick={() => void handleSaveTerminal()}
                disabled={savingTerminal}
              >
                {savingTerminal ? "Saving…" : "Save"}
              </button>
            </div>
          </div>
        </section>

        <section className="detail-section">
          <div className="detail-section-header">
            <span className="detail-section-title">AI Provider</span>
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: 14, padding: "12px 14px" }}>

            {/* Provider selector */}
            <div>
              <label style={{ display: "block", fontSize: 12, color: "var(--text-muted)", marginBottom: 6 }}>
                Provider
              </label>
              <div style={{ display: "flex", gap: 16 }}>
                {(["openai", "anthropic", "local"] as const).map((p) => (
                  <label key={p} style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 13, cursor: "pointer" }}>
                    <input
                      type="radio"
                      name="ai-provider"
                      value={p}
                      checked={provider === p}
                      onChange={() => {
                        setProvider(p);
                        setBaseUrl("");
                        if (p === "openai") setModel("gpt-4.1");
                        else if (p === "anthropic") setModel("claude-sonnet-4-6");
                        else setModel("");
                      }}
                    />
                    {p === "openai" ? "OpenAI" : p === "anthropic" ? "Anthropic (Claude)" : "Local (OpenAI-compatible)"}
                  </label>
                ))}
              </div>
            </div>

            {/* Model selection */}
            <div>
              <label style={{ display: "block", fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>
                Model
              </label>
              {provider === "openai" && (
                <select
                  className="passphrase-input"
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                  style={{ width: "100%" }}
                >
                  {OPENAI_MODELS.map((m) => (
                    <option key={m.id} value={m.id}>{m.label}</option>
                  ))}
                </select>
              )}
              {provider === "anthropic" && (
                <select
                  className="passphrase-input"
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                  style={{ width: "100%" }}
                >
                  {CLAUDE_MODELS.map((m) => (
                    <option key={m.id} value={m.id}>{m.label}</option>
                  ))}
                </select>
              )}
              {provider === "local" && (
                <input
                  className="passphrase-input"
                  placeholder="llama3.2"
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                  style={{ width: "100%" }}
                />
              )}
            </div>

            {/* OpenAI API key — only for openai provider */}
            {provider === "openai" && (
              <div>
                <label style={{ display: "block", fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>
                  OpenAI API key {settings?.hasOpenAIKey ? "(leave empty to keep existing)" : ""}
                </label>
                <input
                  type="password"
                  className="passphrase-input"
                  placeholder={settings?.hasOpenAIKey ? "•••••••••••••••" : "sk-…"}
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  autoComplete="off"
                  style={{ width: "100%" }}
                />
                {settings?.source === "env" && (
                  <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 4 }}>
                    The environment variable <code>OPENAI_API_KEY</code> is active and overrides any key stored here.
                  </div>
                )}
              </div>
            )}

            {/* Anthropic API key — only for anthropic provider */}
            {provider === "anthropic" && (
              <div>
                <label style={{ display: "block", fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>
                  Anthropic API key {settings?.hasAnthropicKey ? "(leave empty to keep existing)" : ""}
                </label>
                <input
                  type="password"
                  className="passphrase-input"
                  placeholder={settings?.hasAnthropicKey ? "•••••••••••••••" : "sk-ant-…"}
                  value={anthropicKey}
                  onChange={(e) => setAnthropicKey(e.target.value)}
                  autoComplete="off"
                  style={{ width: "100%" }}
                />
                {settings?.source === "env" && (
                  <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 4 }}>
                    The environment variable <code>ANTHROPIC_API_KEY</code> is active and overrides any key stored here.
                  </div>
                )}
              </div>
            )}

            {/* Base URL — for openai (optional) and local (required) */}
            {(provider === "openai" || provider === "local") && (
              <div>
                <label style={{ display: "block", fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>
                  Base URL {provider === "local" ? "(required — e.g. http://localhost:11434)" : "(optional — e.g. for proxy or Azure)"}
                </label>
                <input
                  className="passphrase-input"
                  placeholder={provider === "local" ? "http://localhost:11434" : "https://api.openai.com"}
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
                  style={{ width: "100%" }}
                />
              </div>
            )}

            {/* Local optional key */}
            {provider === "local" && (
              <div>
                <label style={{ display: "block", fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>
                  API key (optional)
                </label>
                <input
                  type="password"
                  className="passphrase-input"
                  placeholder="(leave empty if not required)"
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  autoComplete="off"
                  style={{ width: "100%" }}
                />
              </div>
            )}

            <div>
              <button
                className="btn-primary"
                onClick={() => void handleSave()}
                disabled={saving}
              >
                {saving ? "Saving…" : "Save"}
              </button>
            </div>

          </div>
        </section>

        <section className="settings-card">
          <WorkflowStatusManager />
        </section>

      </div>
    </div>
  );
}
