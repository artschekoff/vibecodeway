import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { GetAISettings, SetAISettings } from "../../wailsjs/go/main/App";
import type { AISettings } from "../types";

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

  useEffect(() => {
    let cancelled = false;
    GetAISettings()
      .then((s) => {
        if (cancelled) return;
        const typed = s as unknown as AISettings;
        setSettings(typed);
        setModel(typed.model);
        setBaseUrl(typed.baseUrl);
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
        <button className="back-btn" onClick={() => navigate("/")}>← Back</button>
        <div style={{ flex: 1 }}>
          <h1 style={{ fontSize: 16, margin: 0 }}>Settings · OpenAI</h1>
        </div>
      </div>

      <div className="content" style={{ maxWidth: 620 }}>
        {error && <div className="error-banner">{error}</div>}
        {message && <div className="toast toast--success">{message}</div>}

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
