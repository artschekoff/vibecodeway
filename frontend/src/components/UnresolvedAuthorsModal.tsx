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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-white dark:bg-zinc-900 rounded-xl shadow-2xl w-full max-w-lg max-h-[80vh] flex flex-col">
        <div className="px-6 py-4 border-b border-zinc-200 dark:border-zinc-700">
          <h2 className="text-base font-semibold">Assign Unknown Authors</h2>
          <p className="text-sm text-zinc-500 mt-0.5">
            {unresolved.length} author{unresolved.length !== 1 ? "s" : ""} found in this context with no org assignment.
          </p>
        </div>

        <div className="flex-1 overflow-y-auto px-6 py-4 space-y-4">
          {unresolved.map((author) => (
            <AuthorRow
              key={key(author)}
              author={author}
              orgUsers={orgUsers}
              resolution={resolutions[key(author)]}
              onChange={(r) => setResolution(author, r)}
            />
          ))}
        </div>

        <div className="px-6 py-4 border-t border-zinc-200 dark:border-zinc-700 flex justify-end gap-2">
          <button
            onClick={onClose}
            className="px-4 py-1.5 text-sm rounded-lg border border-zinc-300 dark:border-zinc-600 hover:bg-zinc-50 dark:hover:bg-zinc-800"
          >
            Skip all
          </button>
          <button
            onClick={() => void handleConfirm()}
            disabled={saving}
            className="px-4 py-1.5 text-sm rounded-lg bg-indigo-600 text-white hover:bg-indigo-700 disabled:opacity-50"
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
  orgUsers: store.User[];
  resolution: Resolution | undefined;
  onChange: (r: Resolution) => void;
}

function AuthorRow({ author, orgUsers, resolution, onChange }: AuthorRowProps) {
  const [mode, setMode] = useState<"select" | "create">("select");
  const [newName, setNewName] = useState(author.rawAuthor);
  const [newEmail, setNewEmail] = useState("");

  return (
    <div className="rounded-lg border border-zinc-200 dark:border-zinc-700 p-3 space-y-2">
      <div className="flex items-center gap-2">
        <span className="text-xs uppercase tracking-wide text-zinc-400 w-14 shrink-0">
          {author.sourceLabel}
        </span>
        <span className="font-medium text-sm">{author.rawAuthor}</span>
        <span className="ml-auto text-xs text-zinc-400">
          {author.messageCount} msg{author.messageCount !== 1 ? "s" : ""}
        </span>
      </div>

      <div className="flex gap-2 text-xs">
        <button
          onClick={() => setMode("select")}
          className={`px-2 py-0.5 rounded ${mode === "select" ? "bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300" : "text-zinc-500 hover:text-zinc-700"}`}
        >
          Assign existing
        </button>
        <button
          onClick={() => setMode("create")}
          className={`px-2 py-0.5 rounded ${mode === "create" ? "bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300" : "text-zinc-500 hover:text-zinc-700"}`}
        >
          Create new
        </button>
      </div>

      {mode === "select" && (
        <select
          className="w-full text-sm rounded border border-zinc-300 dark:border-zinc-600 px-2 py-1 bg-white dark:bg-zinc-800"
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
        <div className="space-y-1">
          <input
            className="w-full text-sm rounded border border-zinc-300 dark:border-zinc-600 px-2 py-1 bg-white dark:bg-zinc-800"
            placeholder="Display name"
            value={newName}
            onChange={(e) => {
              setNewName(e.target.value);
              onChange({ type: "new", displayName: e.target.value, email: newEmail });
            }}
          />
          <input
            className="w-full text-sm rounded border border-zinc-300 dark:border-zinc-600 px-2 py-1 bg-white dark:bg-zinc-800"
            placeholder="Email (optional)"
            value={newEmail}
            onChange={(e) => {
              setNewEmail(e.target.value);
              onChange({ type: "new", displayName: newName, email: e.target.value });
            }}
          />
        </div>
      )}
    </div>
  );
}
