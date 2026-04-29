import { useState, useEffect, useRef, useCallback, useMemo } from "react";
import {
  ListAllContexts,
  DismissPicker,
  QuickAttachText,
  CreateContext,
  AddIssuesToContext,
  SetJiraConfig,
  SetJiraBaseURL,
  AddVCSBranch,
} from "../../wailsjs/go/main/App";
import type { Context } from "../types";
import { parseAttachableContent } from "../lib/parseAttachableContent";

interface Props {
  visible: boolean;
  text: string;
  onAttach: (contextKey: string, contextId: string) => void;
  onDismiss: () => void;
}

export const QuickAttachOverlay = ({ visible, text, onAttach, onDismiss }: Props) => {
  const [contexts, setContexts] = useState<Context[]>([]);
  const [filter, setFilter] = useState("");
  const [selectedId, setSelectedId] = useState("");
  const [loading, setLoading] = useState(false);
  const [attaching, setAttaching] = useState(false);
  const [error, setError] = useState("");

  // "+ New Context" inline form state
  const [showNewCtx, setShowNewCtx] = useState(false);
  const [newCtxKey, setNewCtxKey] = useState("");
  const [creatingCtx, setCreatingCtx] = useState(false);
  const [newCtxError, setNewCtxError] = useState("");

  const filterRef = useRef<HTMLInputElement>(null);
  const newCtxInputRef = useRef<HTMLInputElement>(null);
  const focusTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const kind = useMemo(() => parseAttachableContent(text), [text]);

  const handleDismiss = useCallback(() => {
    DismissPicker().catch(() => {});
    onDismiss();
  }, [onDismiss]);

  const loadContexts = useCallback(async () => {
    setLoading(true);
    try {
      const all = await ListAllContexts();
      const active = all.filter((c) => c.status === "active");
      setContexts(active);
      if (active.length === 1) setSelectedId(active[0].id);
    } catch {
      setError("Failed to load contexts.");
    } finally {
      setLoading(false);
      focusTimerRef.current = setTimeout(() => filterRef.current?.focus(), 50);
    }
  }, []);

  useEffect(() => {
    if (!visible) return;
    setFilter("");
    setSelectedId("");
    setError("");
    setAttaching(false);
    setShowNewCtx(false);
    setNewCtxKey("");
    setNewCtxError("");
    void loadContexts();
    return () => {
      if (focusTimerRef.current) {
        clearTimeout(focusTimerRef.current);
        focusTimerRef.current = null;
      }
    };
  }, [visible, loadContexts]);

  useEffect(() => {
    if (!visible) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") handleDismiss();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [visible, handleDismiss]);

  // When the new-context form opens, focus its input.
  useEffect(() => {
    if (showNewCtx) {
      setTimeout(() => newCtxInputRef.current?.focus(), 30);
    }
  }, [showNewCtx]);

  const attachTo = useCallback(
    async (contextID: string, contextKey: string) => {
      switch (kind.type) {
        case "jira":
          // Ensure the jira_provider row exists first, then set base URL and add the issue.
          await SetJiraConfig(contextID, `key = ${kind.key}`);
          await SetJiraBaseURL(contextID, kind.baseUrl);
          await AddIssuesToContext(contextID, [kind.key]);
          break;
        case "vcs_branch":
          await AddVCSBranch(contextID, kind.platform, kind.ownerRepo, kind.branch, "");
          break;
        default:
          await QuickAttachText(contextID, text);
      }
      DismissPicker().catch(() => {});
      onAttach(contextKey, contextID);
    },
    [kind, text, onAttach],
  );

  const handleAttach = useCallback(async () => {
    if (!selectedId) {
      setError("Please select a context.");
      return;
    }
    setAttaching(true);
    setError("");
    try {
      const ctx = contexts.find((c) => c.id === selectedId);
      await attachTo(selectedId, ctx?.key ?? selectedId);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setAttaching(false);
    }
  }, [selectedId, contexts, attachTo]);

  const handleCreateAndAttach = useCallback(async () => {
    const key = newCtxKey.trim();
    if (!key) {
      setNewCtxError("Context name is required.");
      return;
    }
    setCreatingCtx(true);
    setNewCtxError("");
    try {
      const ctx = await CreateContext(key);
      setContexts((prev) => [...prev, ctx]);
      setSelectedId(ctx.id);
      setShowNewCtx(false);
      setNewCtxKey("");
      // Attach immediately after creating.
      await attachTo(ctx.id, ctx.key);
    } catch (e) {
      setNewCtxError(e instanceof Error ? e.message : String(e));
    } finally {
      setCreatingCtx(false);
    }
  }, [newCtxKey, attachTo]);

  if (!visible) return null;

  const trimmedFilter = filter.trim().toLowerCase();
  const filtered = trimmedFilter
    ? contexts.filter((c) => c.key.toLowerCase().includes(trimmedFilter))
    : contexts;

  // Contextual header based on URL detection.
  const renderPreview = () => {
    switch (kind.type) {
      case "jira":
        return (
          <div className="quick-attach-preview quick-attach-preview--url">
            Add Jira issue <strong>{kind.key}</strong>
            <span className="quick-attach-url-host">{kind.baseUrl}</span>
          </div>
        );
      case "vcs_branch":
        return (
          <div className="quick-attach-preview quick-attach-preview--url">
            Link{kind.branch ? " branch" : " repo"}{" "}
            <strong>{kind.ownerRepo}{kind.branch ? ` : ${kind.branch}` : ""}</strong>
            <span className="quick-attach-url-host">{kind.platform}</span>
          </div>
        );
      default: {
        const chars = [...text];
        const preview = chars.length > 240 ? chars.slice(0, 240).join("") + "…" : text;
        return <div className="quick-attach-preview">{preview}</div>;
      }
    }
  };

  const attachLabel =
    kind.type === "jira" ? "Add Issue" :
    kind.type === "vcs_branch" ? (kind.branch ? "Link Branch" : "Link Repo") :
    "Attach";

  return (
    <div
      className="modal-overlay quick-attach-overlay"
      onClick={(e) => {
        if (e.target === e.currentTarget) handleDismiss();
      }}
    >
      <div className="modal quick-attach-modal">
        <h2>
          {kind.type === "jira" ? "Add Jira issue to context" :
           kind.type === "vcs_branch" ? "Link repository to context" :
           "Attach text to context"}
        </h2>

        {renderPreview()}

        <div className="quick-attach-filter-row">
          <input
            ref={filterRef}
            className="field-input quick-attach-filter"
            placeholder="Filter contexts…"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            disabled={attaching || creatingCtx}
          />
        </div>

        <div className="quick-attach-list">
          {loading ? (
            <div className="quick-attach-empty">Loading contexts…</div>
          ) : filtered.length === 0 && !showNewCtx ? (
            <div className="quick-attach-empty">
              {filter ? "No contexts match your filter." : "No active contexts found."}
            </div>
          ) : (
            filtered.map((c) => (
              <label
                key={c.id}
                className={`quick-attach-item${selectedId === c.id ? " quick-attach-item--selected" : ""}`}
              >
                <input
                  type="radio"
                  name="quick-attach-context"
                  value={c.id}
                  checked={selectedId === c.id}
                  onChange={() => setSelectedId(c.id)}
                  disabled={attaching || creatingCtx}
                />
                <span className="quick-attach-item-key">{c.key}</span>
              </label>
            ))
          )}
        </div>

        {/* "+ New Context" form */}
        {showNewCtx ? (
          <div className="quick-attach-new-ctx">
            <input
              ref={newCtxInputRef}
              className="field-input quick-attach-new-ctx-input"
              placeholder="Context name…"
              value={newCtxKey}
              onChange={(e) => setNewCtxKey(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") { e.preventDefault(); void handleCreateAndAttach(); }
                if (e.key === "Escape") { setShowNewCtx(false); setNewCtxKey(""); setNewCtxError(""); }
              }}
              disabled={creatingCtx}
            />
            <button
              className="btn-primary"
              style={{ fontSize: 12, padding: "3px 10px", whiteSpace: "nowrap" }}
              onClick={() => void handleCreateAndAttach()}
              disabled={creatingCtx || !newCtxKey.trim()}
            >
              {creatingCtx ? "Creating…" : `Create & ${attachLabel}`}
            </button>
            <button
              className="btn-secondary"
              style={{ fontSize: 12, padding: "3px 8px" }}
              onClick={() => { setShowNewCtx(false); setNewCtxKey(""); setNewCtxError(""); }}
              disabled={creatingCtx}
            >
              Cancel
            </button>
            {newCtxError && <div className="field-error" style={{ marginTop: 4 }}>{newCtxError}</div>}
          </div>
        ) : (
          <button
            className="quick-attach-new-ctx-btn"
            onClick={() => setShowNewCtx(true)}
            disabled={attaching}
          >
            + New Context
          </button>
        )}

        {error && <div className="field-error">{error}</div>}

        <div className="modal-footer">
          <button className="btn-secondary" onClick={handleDismiss} disabled={attaching || creatingCtx}>
            Cancel
          </button>
          <button
            className="btn-primary"
            onClick={() => { void handleAttach(); }}
            disabled={attaching || creatingCtx || !selectedId || loading}
          >
            {attaching ? `${attachLabel}ing…` : attachLabel}
          </button>
        </div>
      </div>
    </div>
  );
};
