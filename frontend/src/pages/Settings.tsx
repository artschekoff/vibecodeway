import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { ArrowLeft, Copy, Check } from "lucide-react";
import { GetAISettings, SetAISettings, GetSupportedPlatforms, GetVCSAuth, VCSLoginStart, VCSLoginComplete } from "../../wailsjs/go/main/App";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import type { AISettings, VCSPlatformInfo, VCSPlatform, DeviceCodeInfo } from "../types";

export default function SettingsPage() {
  const navigate = useNavigate();
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

  useEffect(() => {
    let cancelled = false;
    Promise.all([
      GetAISettings(),
      GetSupportedPlatforms().catch(() => []),
    ])
      .then(([s, plats]) => {
        if (cancelled) return;
        const typed = s as unknown as AISettings;
        setSettings(typed);
        setModel(typed.model);
        setBaseUrl(typed.baseUrl);

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
        <button className="back-btn" onClick={() => navigate("/")}><ArrowLeft size={16} /> Back</button>
        <div style={{ flex: 1 }}>
          <h1 style={{ fontSize: 16, margin: 0 }}>Settings · OpenAI</h1>
        </div>
      </div>

      <div className="content" style={{ maxWidth: 620 }}>
        {error && <div className="error-banner">{error}</div>}
        {message && <div className="toast toast--success">{message}</div>}

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
