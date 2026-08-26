import { useEffect, useState, useCallback } from "react";
import { RotateCcw, Save } from "lucide-react";
import { ListAIPrompts, UpdateAIPrompt, ResetAIPrompt } from "../../wailsjs/go/main/App";
import type { store } from "../../wailsjs/go/models";
import { ProseEditor } from "../components/ProseEditor";

interface PromptState {
  body: string;
  saving: boolean;
  resetting: boolean;
  saved: boolean;
  confirmReset: boolean;
  error: string;
}

function buildStates(prompts: store.AIPrompt[]): Record<string, PromptState> {
  return Object.fromEntries(
    prompts.map((p) => [
      p.id,
      { body: p.body, saving: false, resetting: false, saved: false, confirmReset: false, error: "" },
    ])
  );
}

export default function PromptsPage() {
  const [prompts, setPrompts] = useState<store.AIPrompt[]>([]);
  const [states, setStates] = useState<Record<string, PromptState>>({});
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");

  const load = useCallback(() => {
    setLoading(true);
    setLoadError("");
    ListAIPrompts()
      .then((data) => {
        const list = data ?? [];
        setPrompts(list);
        setStates(buildStates(list));
      })
      .catch((e) => setLoadError(String(e)))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => { load(); }, [load]);

  const patchState = (id: string, patch: Partial<PromptState>) =>
    setStates((prev) => ({ ...prev, [id]: { ...prev[id], ...patch } }));

  const handleBodyChange = (id: string, value: string) =>
    patchState(id, { body: value, saved: false });

  const handleSave = async (id: string) => {
    patchState(id, { saving: true, error: "", saved: false });
    try {
      await UpdateAIPrompt(id, states[id].body);
      patchState(id, { saving: false, saved: true });
      setTimeout(() => patchState(id, { saved: false }), 2000);
    } catch (e) {
      patchState(id, { saving: false, error: String(e) });
    }
  };

  const handleReset = async (id: string) => {
    const st = states[id];
    if (!st.confirmReset) {
      patchState(id, { confirmReset: true });
      setTimeout(() => patchState(id, { confirmReset: false }), 3000);
      return;
    }
    patchState(id, { resetting: true, confirmReset: false, error: "" });
    try {
      await ResetAIPrompt(id);
      const refreshed = await ListAIPrompts();
      const updated = (refreshed ?? []).find((p) => p.id === id);
      if (updated) patchState(id, { resetting: false, body: updated.body, saved: false });
      else patchState(id, { resetting: false });
    } catch (e) {
      patchState(id, { resetting: false, error: String(e) });
    }
  };

  return (
    <div className="page">
      <div className="topbar">
        <div style={{ flex: 1 }}>
          <h1 style={{ fontSize: 16, margin: 0 }}>AI Prompts</h1>
        </div>
      </div>

      <div className="content" style={{ padding: "16px 20px", display: "flex", flexDirection: "column", gap: 24 }}>
        {loadError && <div className="error-banner">{loadError}</div>}

        {loading ? (
          <div className="settings-logs-empty">Loading…</div>
        ) : prompts.length === 0 ? (
          <div className="settings-logs-empty">No prompts found.</div>
        ) : (
          prompts.map((prompt) => {
            const st = states[prompt.id];
            if (!st) return null;
            return (
              <div key={prompt.id} className="ai-prompt-card">
                <div className="ai-prompt-card__header">
                  <span className="ai-prompt-card__label">{prompt.label}</span>
                  <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
                    {st.error && (
                      <span style={{ fontSize: 11, color: "var(--danger)" }}>{st.error}</span>
                    )}
                    {st.saved && (
                      <span style={{ fontSize: 11, color: "var(--success, #4caf50)" }}>Saved</span>
                    )}
                    <button
                      className={st.confirmReset ? "btn-danger" : "btn-secondary"}
                      style={{ fontSize: 12, padding: "3px 8px", display: "flex", alignItems: "center", gap: 4 }}
                      onClick={() => void handleReset(prompt.id)}
                      disabled={st.resetting || st.saving}
                      title="Reset to default"
                    >
                      <RotateCcw size={12} />
                      {st.confirmReset ? "Confirm reset" : "Reset"}
                    </button>
                    <button
                      className="btn-primary"
                      style={{ fontSize: 12, padding: "3px 8px", display: "flex", alignItems: "center", gap: 4 }}
                      onClick={() => void handleSave(prompt.id)}
                      disabled={st.saving || st.resetting}
                    >
                      <Save size={12} />
                      {st.saving ? "Saving…" : "Save"}
                    </button>
                  </div>
                </div>

                <ProseEditor
                  className="ai-prompt-card__textarea"
                  value={st.body}
                  onChange={(e) => handleBodyChange(prompt.id, e.target.value)}
                  spellCheck={false}
                />

                {prompt.placeholders?.length > 0 && (
                  <div className="ai-prompt-card__placeholders">
                    <span className="ai-prompt-card__placeholders-title">Available placeholders</span>
                    <div className="ai-prompt-card__placeholder-list">
                      {prompt.placeholders.map((ph) => (
                        <div key={ph.name} className="ai-prompt-card__placeholder-item">
                          <code className="ai-prompt-card__placeholder-name">{`{{${ph.name}}}`}</code>
                          <span className="ai-prompt-card__placeholder-desc">{ph.description}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}
