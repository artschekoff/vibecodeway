import { useState, useEffect, useRef, useCallback } from "react";
import { ListAllContexts, DismissPicker, QuickAttachText } from "../../wailsjs/go/main/App";
import type { Context } from "../types";

interface Props {
  visible: boolean;
  text: string;
  onAttach: (contextKey: string) => void;
  onDismiss: () => void;
}

export const QuickAttachOverlay = ({ visible, text, onAttach, onDismiss }: Props) => {
  const [contexts, setContexts] = useState<Context[]>([]);
  const [filter, setFilter] = useState("");
  const [selectedId, setSelectedId] = useState("");
  const [loading, setLoading] = useState(false);
  const [attaching, setAttaching] = useState(false);
  const [error, setError] = useState("");
  const filterRef = useRef<HTMLInputElement>(null);
  const focusTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const handleDismiss = useCallback(() => {
    DismissPicker().catch(() => {}); // best-effort: clears always-on-top; ignore if bridge is gone
    onDismiss();
  }, [onDismiss]);

  useEffect(() => {
    if (!visible) return;
    setFilter("");
    setSelectedId("");
    setError("");
    setAttaching(false);
    setLoading(true);
    ListAllContexts()
      .then((all) => {
        const active = all.filter((c) => c.status === "active");
        setContexts(active);
        if (active.length === 1) setSelectedId(active[0].id);
      })
      .catch(() => setError("Failed to load contexts."))
      .finally(() => {
        setLoading(false);
        // Focus the filter after a short delay so the overlay is fully rendered.
        focusTimerRef.current = setTimeout(() => filterRef.current?.focus(), 50);
      });
    return () => {
      if (focusTimerRef.current) {
        clearTimeout(focusTimerRef.current);
        focusTimerRef.current = null;
      }
    };
  }, [visible]);

  useEffect(() => {
    if (!visible) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") handleDismiss();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [visible, handleDismiss]);

  const handleAttach = useCallback(async () => {
    if (!selectedId) {
      setError("Please select a context.");
      return;
    }
    setAttaching(true);
    setError("");
    try {
      await QuickAttachText(selectedId, text);
      const ctx = contexts.find((c) => c.id === selectedId);
      onAttach(ctx?.key ?? selectedId);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setAttaching(false);
    }
  }, [selectedId, text, contexts, onAttach]);

  if (!visible) return null;

  // Use trimmed filter for both empty-check and predicate to avoid leading/trailing-space mismatches.
  const trimmedFilter = filter.trim().toLowerCase();
  const filtered = trimmedFilter
    ? contexts.filter((c) => c.key.toLowerCase().includes(trimmedFilter))
    : contexts;

  // Use spread to slice by Unicode codepoint, not UTF-16 code unit, to avoid splitting surrogate pairs.
  const chars = [...text];
  const preview = chars.length > 240 ? chars.slice(0, 240).join("") + "…" : text;

  return (
    <div
      className="modal-overlay quick-attach-overlay"
      onClick={(e) => {
        if (e.target === e.currentTarget) handleDismiss();
      }}
    >
      <div className="modal quick-attach-modal">
        <h2>Attach text to context</h2>

        <div className="quick-attach-preview">{preview}</div>

        <div className="quick-attach-filter-row">
          <input
            ref={filterRef}
            className="field-input quick-attach-filter"
            placeholder="Filter contexts…"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            disabled={attaching}
          />
        </div>

        <div className="quick-attach-list">
          {loading ? (
            <div className="quick-attach-empty">Loading contexts…</div>
          ) : filtered.length === 0 ? (
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
                  disabled={attaching}
                />
                <span className="quick-attach-item-key">{c.key}</span>
              </label>
            ))
          )}
        </div>

        {error && <div className="field-error">{error}</div>}

        <div className="modal-footer">
          <button className="btn-secondary" onClick={handleDismiss} disabled={attaching}>
            Cancel
          </button>
          <button
            className="btn-primary"
            onClick={() => {
              void handleAttach();
            }}
            disabled={attaching || !selectedId || loading}
          >
            {attaching ? "Attaching…" : "Attach"}
          </button>
        </div>
      </div>
    </div>
  );
};

