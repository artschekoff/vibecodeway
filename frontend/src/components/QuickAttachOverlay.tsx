import { useState, useEffect, useRef, useCallback, useMemo } from "react";
import {
  ListAllContexts,
  ListOrganizations,
  DismissPicker,
  QuickAttachText,
  CreateContext,
  AddIssuesToContext,
  SetJiraConfig,
  SetJiraBaseURL,
  AddVCSBranch,
  AddSpecificPR,
  AddUserEvent,
} from "../../wailsjs/go/main/App";
import type { Context, Organization } from "../types";
import { parseAttachableContent } from "../lib/parseAttachableContent";

interface Props {
  visible: boolean;
  text: string;
  orgId?: string;
  onAttach: (contextKey: string, contextId: string, orgId?: string, orgName?: string) => void;
  onDismiss: () => void;
}

export const QuickAttachOverlay = ({ visible, text, orgId, onAttach, onDismiss }: Props) => {
  const [contexts, setContexts] = useState<Context[]>([]);
  const [filter, setFilter] = useState("");
  const [selectedId, setSelectedId] = useState("");
  const [loading, setLoading] = useState(false);
  const [attaching, setAttaching] = useState(false);
  const [error, setError] = useState("");

  type Destination = "text" | "log";
  const [destination, setDestination] = useState<Destination>("text");
  const [logDraft, setLogDraft] = useState("");

  // "+ New Context" inline form state
  const [showNewCtx, setShowNewCtx] = useState(false);
  const [newCtxKey, setNewCtxKey] = useState("");
  const [newCtxOrgId, setNewCtxOrgId] = useState("");
  const [orgs, setOrgs] = useState<Organization[]>([]);
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
      const [all, allOrgs] = await Promise.all([ListAllContexts(), ListOrganizations()]);
      const active = all.filter((c) => c.status === "active" && (!orgId || c.orgId === orgId));
      setContexts(active);
      setOrgs(allOrgs ?? []);
      if (active.length === 1) setSelectedId(active[0].id);
    } catch {
      setError("Failed to load contexts.");
    } finally {
      setLoading(false);
      focusTimerRef.current = setTimeout(() => filterRef.current?.focus(), 50);
    }
  }, [orgId]);

  useEffect(() => {
    if (!visible) return;
    setFilter("");
    setSelectedId("");
    setError("");
    setAttaching(false);
    setShowNewCtx(false);
    setNewCtxKey("");
    setNewCtxOrgId("");
    setNewCtxError("");
    setDestination("text");
    setLogDraft("");
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
    async (contextID: string, contextKey: string, knownOrgId?: string) => {
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
        case "vcs_pr":
          await AddSpecificPR(contextID, kind.platform, kind.ownerRepo, kind.prNumber);
          break;
        default:
          if (destination === "log") {
            await AddUserEvent(contextID, "user_note", logDraft.trim() || text);
          } else {
            await QuickAttachText(contextID, text);
          }
      }
      const ctx = contexts.find((c) => c.id === contextID);
      const resolvedOrgId = knownOrgId ?? ctx?.orgId;
      const ctxOrgName = orgs.find((o) => o.id === resolvedOrgId)?.name;
      onAttach(contextKey, contextID, resolvedOrgId, ctxOrgName);
    },
    [kind, text, destination, logDraft, onAttach, contexts, orgs],
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
      // Dismiss after onAttach has queued setPicker(false) — wait one animation
      // frame so React has unmounted the modal before [NSApp hide:nil] fires,
      // preventing the macOS hide animation from capturing a half-rendered modal.
      requestAnimationFrame(() => DismissPicker().catch(() => {}));
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
    const resolvedOrgId = orgId ?? newCtxOrgId;
    if (!resolvedOrgId) {
      setNewCtxError("Select an organization.");
      return;
    }
    setCreatingCtx(true);
    setNewCtxError("");
    try {
      const ctx = await CreateContext(key, resolvedOrgId);
      setContexts((prev) => [...prev, ctx]);
      setSelectedId(ctx.id);
      setShowNewCtx(false);
      setNewCtxKey("");
      setNewCtxOrgId("");
      // Attach immediately after creating, then dismiss (same pattern as handleAttach).
      await attachTo(ctx.id, ctx.key, resolvedOrgId);
      requestAnimationFrame(() => DismissPicker().catch(() => {}));
    } catch (e) {
      setNewCtxError(e instanceof Error ? e.message : String(e));
    } finally {
      setCreatingCtx(false);
    }
  }, [newCtxKey, orgId, newCtxOrgId, attachTo]);

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
      case "vcs_pr":
        return (
          <div className="quick-attach-preview quick-attach-preview--url">
            Add PR <strong>#{kind.prNumber}</strong> from <strong>{kind.ownerRepo}</strong>
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
    kind.type === "vcs_pr" ? "Add PR" :
    destination === "log" ? "Add to Log" :
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
           kind.type === "vcs_pr" ? "Add pull request to context" :
           "Attach text to context"}
        </h2>

        {renderPreview()}

        {kind.type === "text" && (
          <div className="quick-attach-destination">
            <label className={`quick-attach-dest-option${destination === "text" ? " quick-attach-dest-option--selected" : ""}`}>
              <input
                type="radio"
                name="destination"
                value="text"
                checked={destination === "text"}
                onChange={() => { setDestination("text"); setLogDraft(""); }}
              />
              Add to Notes
            </label>
            <label className={`quick-attach-dest-option${destination === "log" ? " quick-attach-dest-option--selected" : ""}`}>
              <input
                type="radio"
                name="destination"
                value="log"
                checked={destination === "log"}
                onChange={() => { setDestination("log"); setLogDraft(text); }}
              />
              Add to Log
            </label>
          </div>
        )}

        {kind.type === "text" && destination === "log" && (
          <div className="quick-attach-log-draft">
            <textarea
              className="chat-input"
              value={logDraft}
              onChange={(e) => setLogDraft(e.target.value)}
              rows={5}
              placeholder="Edit before saving as log entry…"
            />
          </div>
        )}

        <div className="quick-attach-filter-row">
          <input
            ref={filterRef}
            className="field-input quick-attach-filter"
            placeholder="Filter contexts…"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            disabled={attaching || creatingCtx}
            autoCorrect="off"
            autoCapitalize="none"
            spellCheck={false}
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
            {!orgId && orgs.length > 0 && (
              <select
                className="field-input quick-attach-new-ctx-org"
                value={newCtxOrgId}
                onChange={(e) => setNewCtxOrgId(e.target.value)}
                disabled={creatingCtx}
              >
                <option value="">Select organization…</option>
                {orgs.map((o) => (
                  <option key={o.id} value={o.id}>{o.name}</option>
                ))}
              </select>
            )}
            <input
              ref={newCtxInputRef}
              className="field-input quick-attach-new-ctx-input"
              placeholder="Context name…"
              value={newCtxKey}
              onChange={(e) => setNewCtxKey(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") { e.preventDefault(); void handleCreateAndAttach(); }
                if (e.key === "Escape") { setShowNewCtx(false); setNewCtxKey(""); setNewCtxOrgId(""); setNewCtxError(""); }
              }}
              disabled={creatingCtx}
              autoCorrect="off"
              autoCapitalize="none"
              spellCheck={false}
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
              onClick={() => { setShowNewCtx(false); setNewCtxKey(""); setNewCtxOrgId(""); setNewCtxError(""); }}
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
            className={`btn-primary${attaching ? " btn-with-spinner" : ""}`}
            onClick={() => { void handleAttach(); }}
            disabled={attaching || creatingCtx || !selectedId || loading}
          >
            {attaching && (
              <svg
                width="14" height="14" viewBox="0 0 14 14" fill="none"
                style={{ animation: "progress-spin 0.8s linear infinite", flexShrink: 0 }}
              >
                <circle cx="7" cy="7" r="5.5" stroke="currentColor" strokeOpacity="0.3" strokeWidth="2" />
                <path d="M7 1.5A5.5 5.5 0 0 1 12.5 7" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
              </svg>
            )}
            {attaching ? `${attachLabel}ing…` : attachLabel}
          </button>
        </div>
      </div>
    </div>
  );
};
