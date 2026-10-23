import { useCallback, useEffect, useState } from "react";
import { Trash2 } from "lucide-react";
import { AddUserEvent, DeleteUserEvent, ListEvents } from "../../../wailsjs/go/main/App";
import type { store } from "../../../wailsjs/go/models";
import { ProseEditor } from "../ProseEditor";

const NOTE_TYPE = "user_note";

// ponytail: notes are just user_note events — no new table, no new bindings.
// The chain is per-context, so the same log shows up wherever the context does.

function noteTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
}

/**
 * Append-only scratch log for the context, in the editor sidebar: jot what you
 * just did before switching tasks, read it back when you return.
 *
 * Collapsed, the panel is just the latest note — the sidebar is short and an
 * always-open composer left the log one clipped line. Clicking it opens the
 * composer, caret inside, with the previous notes small and muted underneath;
 * adding a note (⌘Enter, or Esc then Enter in vim mode) collapses back so the
 * thing you just wrote is what you see.
 */
export function NotesPanel({ contextId }: { contextId: string }) {
  const [notes, setNotes] = useState<store.Event[]>([]);
  const [draft, setDraft] = useState("");
  const [saving, setSaving] = useState(false);
  const [open, setOpen] = useState(false);

  const load = useCallback(() => {
    return ListEvents(contextId)
      .then((rows) => setNotes((rows ?? []).filter((e) => e.type === NOTE_TYPE).reverse()))
      .catch(() => setNotes([]));
  }, [contextId]);

  useEffect(() => {
    let active = true;
    void ListEvents(contextId)
      .then((rows) => { if (active) setNotes((rows ?? []).filter((e) => e.type === NOTE_TYPE).reverse()); })
      .catch(() => { if (active) setNotes([]); });
    return () => { active = false; };
  }, [contextId]);

  // Switching contexts should not carry an open composer — or a draft — across.
  useEffect(() => { setOpen(false); setDraft(""); }, [contextId]);

  const add = async () => {
    const text = draft.trim();
    if (!text || saving) return;
    setSaving(true);
    try {
      await AddUserEvent(contextId, NOTE_TYPE, text);
      setDraft("");
      await load();
      setOpen(false);
    } catch {
      // keep the draft so nothing typed is lost
    } finally {
      setSaving(false);
    }
  };

  const remove = async (id: string) => {
    try {
      await DeleteUserEvent(contextId, id);
      await load();
    } catch {
      // best-effort
    }
  };

  const latest = notes[0];

  if (!open) {
    return (
      <button
        onClick={() => setOpen(true)}
        aria-label="open notes"
        aria-expanded={false}
        style={{ display: "block", width: "100%", textAlign: "left", background: "none",
          border: "none", cursor: "pointer", padding: "8px 12px", font: "inherit" }}
      >
        {latest ? (
          <>
            <div style={{ fontSize: 10, color: "var(--text-muted)", marginBottom: 2 }}>{noteTime(latest.timestamp)}</div>
            {/* The latest note is the "where was I?" answer — bigger + accent so
                it doesn't drown among the sidebar's muted labels. */}
            <div style={{ fontSize: 14, fontWeight: 600, color: "var(--accent, #e3b341)", whiteSpace: "pre-wrap", wordBreak: "break-word",
              display: "-webkit-box", WebkitLineClamp: 3, WebkitBoxOrient: "vertical", overflow: "hidden" }}>
              {latest.summary}
            </div>
          </>
        ) : (
          <div style={{ fontSize: 12, color: "var(--text-muted)" }}>What did you just do here?</div>
        )}
      </button>
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", minHeight: 0, flex: 1 }}>
      <div style={{ padding: 8, borderBottom: "1px solid var(--border)", display: "flex", flexDirection: "column", gap: 6 }}>
        <ProseEditor
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
              e.preventDefault();
              void add();
            }
            // Escape with text in the box belongs to vim (INSERT → NORMAL, then
            // Enter submits); only an empty composer closes on it.
            if (e.key === "Escape" && draft.trim() === "") {
              e.preventDefault();
              setOpen(false);
            }
          }}
          onEnterSubmit={() => void add()}
          placeholder="What did you just do here?"
          aria-label="note text"
          autoFocus
          rows={3}
          style={{ resize: "vertical", padding: 6,
            background: "var(--surface2)", color: "var(--text)", border: "1px solid var(--border)", borderRadius: 4 }}
        />
        <div style={{ display: "flex", alignSelf: "flex-end", gap: 6 }}>
          <button
            onClick={() => setOpen(false)}
            aria-label="close notes"
            style={{ fontSize: 11, padding: "4px 10px", borderRadius: 4, background: "none",
              color: "var(--text-muted)", border: "1px solid var(--border)", cursor: "pointer" }}
          >
            Close
          </button>
          <button
            onClick={() => void add()}
            disabled={saving || draft.trim() === ""}
            aria-label="add note"
            style={{ fontSize: 11, padding: "4px 10px", borderRadius: 4,
              background: "var(--surface2)", color: "var(--text)", border: "1px solid var(--border)",
              cursor: saving || draft.trim() === "" ? "default" : "pointer",
              opacity: saving || draft.trim() === "" ? 0.5 : 1 }}
          >
            Add note
          </button>
        </div>
      </div>

      <div style={{ overflowY: "auto", flex: 1 }}>
        {notes.length === 0 ? (
          <div style={{ fontSize: 12, color: "var(--text-muted)", padding: 12 }}>No notes yet.</div>
        ) : (
          notes.map((n, i) => (
            <div key={n.id} style={{ padding: "8px 12px", borderBottom: "1px solid var(--border)", display: "flex", gap: 8, alignItems: "flex-start" }}>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontSize: 10, color: "var(--text-muted)", marginBottom: 2 }}>{noteTime(n.timestamp)}</div>
                {/* While composing, what's below is the past: the previous note
                    sits right under the box, small and muted, older ones fainter. */}
                <div style={{ fontSize: 11, color: "var(--text-muted)", opacity: i === 0 ? 1 : 0.7,
                  whiteSpace: "pre-wrap", wordBreak: "break-word" }}>{n.summary}</div>
              </div>
              <button
                onClick={() => void remove(n.id)}
                title="Delete note"
                aria-label={`delete note ${n.id}`}
                style={{ background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)", padding: 2 }}
              >
                <Trash2 size={12} />
              </button>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
