import { useState, useEffect } from "react";
import {
  GetUnresolvedAuthors,
  AssignAuthorBatch,
  CreateUserAndAssign,
  ListOrgUsers,
} from "../../wailsjs/go/main/App";
import type { store } from "../../wailsjs/go/models";

interface Props {
  contextID: string;
  orgID: string;
  onClose: () => void;
}

type Resolution =
  | { type: "existing"; userID: string }
  | { type: "new"; displayName: string; email: string }
  | { type: "skip" };

export function UnresolvedAuthorsModal({ contextID, orgID, onClose }: Props) {
  const [unresolved, setUnresolved] = useState<store.UnresolvedAuthor[]>([]);
  const [orgUsers, setOrgUsers] = useState<store.User[]>([]);
  const [resolutions, setResolutions] = useState<Record<string, Resolution>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    Promise.all([
      GetUnresolvedAuthors(contextID),
      ListOrgUsers(orgID),
    ]).then(([authors, users]) => {
      setUnresolved(authors ?? []);
      setOrgUsers(users ?? []);
      setLoading(false);
    });
  }, [contextID, orgID]);

  const key = (a: store.UnresolvedAuthor) => `${a.sourceId}::${a.rawAuthor}`;

  const setResolution = (a: store.UnresolvedAuthor, r: Resolution) =>
    setResolutions((prev) => ({ ...prev, [key(a)]: r }));

  const handleConfirm = async () => {
    setSaving(true);
    const assignments: store.AuthorAssignment[] = [];

    for (const author of unresolved) {
      const r = resolutions[key(author)];
      if (!r || r.type === "skip") continue;

      if (r.type === "existing") {
        assignments.push({
          sourceId: author.sourceId,
          rawAuthor: author.rawAuthor,
          userId: r.userID,
        } as store.AuthorAssignment);
      } else if (r.type === "new" && r.displayName.trim()) {
        const newUser = await CreateUserAndAssign(
          r.displayName.trim(),
          r.email,
          orgID,
          author.sourceId,
          author.rawAuthor,
        );
        if (newUser) {
          setOrgUsers((prev) => [...prev, newUser]);
        }
      }
    }

    if (assignments.length > 0) {
      await AssignAuthorBatch(assignments);
    }
    setSaving(false);
    onClose();
  };

  if (loading) return null;
  if (unresolved.length === 0) { onClose(); return null; }

  return (
    <div className="modal-overlay">
      <div className="modal" style={{ width: 520 }}>
        <h2>Assign Unknown Authors</h2>
        <p style={{ fontSize: 13, color: "var(--text-muted)", marginTop: -12, marginBottom: 16 }}>
          {unresolved.length} author{unresolved.length !== 1 ? "s" : ""} found in this context with no org assignment.
        </p>

        <div className="modal-scroll-body">
          {unresolved.map((author) => (
            <AuthorRow
              key={key(author)}
              author={author}
              orgID={orgID}
              orgUsers={orgUsers}
              resolution={resolutions[key(author)]}
              onChange={(r) => setResolution(author, r)}
              onUserCreated={(u) => setOrgUsers((prev) => [...prev, u])}
            />
          ))}
        </div>

        <div className="modal-footer">
          <button onClick={onClose} className="btn-secondary">
            Skip all
          </button>
          <button
            onClick={() => void handleConfirm()}
            disabled={saving}
            className="btn-primary"
          >
            {saving ? "Saving…" : "Confirm"}
          </button>
        </div>
      </div>
    </div>
  );
}

interface AuthorRowProps {
  author: store.UnresolvedAuthor;
  orgID: string;
  orgUsers: store.User[];
  resolution: Resolution | undefined;
  onChange: (r: Resolution) => void;
  onUserCreated: (user: store.User) => void;
}

function AuthorRow({ author, orgID, orgUsers, resolution, onChange, onUserCreated }: AuthorRowProps) {
  const [mode, setMode] = useState<"select" | "create">("select");
  const [newName, setNewName] = useState(author.rawAuthor);
  const [newEmail, setNewEmail] = useState("");
  const [creating, setCreating] = useState(false);
  const [created, setCreated] = useState(false);

  return (
    <div style={{
      border: "1px solid var(--border)",
      borderRadius: "var(--radius-sm)",
      padding: "10px 12px",
      marginBottom: 10,
      display: "flex",
      flexDirection: "column",
      gap: 8,
    }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <span style={{
          fontSize: 10,
          textTransform: "uppercase",
          letterSpacing: "0.05em",
          color: "var(--text-muted)",
          width: 40,
          flexShrink: 0,
        }}>
          {author.sourceLabel}
        </span>
        <span style={{ fontWeight: 500, fontSize: 13 }}>{author.rawAuthor}</span>
        <span style={{ marginLeft: "auto", fontSize: 11, color: "var(--text-muted)" }}>
          {author.messageCount} msg{author.messageCount !== 1 ? "s" : ""}
        </span>
      </div>

      <div style={{ display: "flex", gap: 6 }}>
        <button
          onClick={() => {
            setMode("select");
            if (!resolution || resolution.type === "new") onChange({ type: "skip" });
          }}
          style={{
            padding: "2px 10px",
            fontSize: 12,
            background: mode === "select" ? "var(--accent)" : "var(--surface2)",
            color: mode === "select" ? "#fff" : "var(--text-muted)",
          }}
        >
          Assign existing
        </button>
        <button
          onClick={() => {
            setMode("create");
            onChange({ type: "new", displayName: newName, email: newEmail });
          }}
          style={{
            padding: "2px 10px",
            fontSize: 12,
            background: mode === "create" ? "var(--accent)" : "var(--surface2)",
            color: mode === "create" ? "#fff" : "var(--text-muted)",
          }}
        >
          Create new
        </button>
      </div>

      {mode === "select" && (
        <select
          style={{ width: "100%" }}
          value={resolution?.type === "existing" ? resolution.userID : ""}
          onChange={(e) =>
            e.target.value
              ? onChange({ type: "existing", userID: e.target.value })
              : onChange({ type: "skip" })
          }
        >
          <option value="">— skip for now —</option>
          {orgUsers.map((u) => (
            <option key={u.id} value={u.id}>
              {u.displayName}
              {u.email ? ` (${u.email})` : ""}
            </option>
          ))}
        </select>
      )}

      {mode === "create" && (
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          {created ? (
            <span style={{ fontSize: 12, color: "var(--text-muted)" }}>
              ✓ Created
            </span>
          ) : (
            <>
              <input
                placeholder="Display name"
                value={newName}
                disabled={creating}
                onChange={(e) => {
                  setNewName(e.target.value);
                  onChange({ type: "new", displayName: e.target.value, email: newEmail });
                }}
              />
              <input
                placeholder="Email (optional)"
                value={newEmail}
                disabled={creating}
                onChange={(e) => {
                  setNewEmail(e.target.value);
                  onChange({ type: "new", displayName: newName, email: e.target.value });
                }}
              />
              <button
                className="btn-primary"
                style={{ alignSelf: "flex-end", padding: "3px 14px", fontSize: 12 }}
                disabled={creating || !newName.trim()}
                onClick={async () => {
                  setCreating(true);
                  const newUser = await CreateUserAndAssign(
                    newName.trim(),
                    newEmail,
                    orgID,
                    author.sourceId,
                    author.rawAuthor,
                  );
                  if (newUser) {
                    onUserCreated(newUser);
                    onChange({ type: "skip" });
                    setCreated(true);
                  }
                  setCreating(false);
                }}
              >
                {creating ? "Creating…" : "Create"}
              </button>
            </>
          )}
        </div>
      )}
    </div>
  );
}
