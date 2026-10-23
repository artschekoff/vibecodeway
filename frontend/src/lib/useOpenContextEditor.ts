import { useState } from "react";
import { EnsureContextEditorWorkspace, CheckoutInWorkspace, EnsureBranchWorktree } from "../../wailsjs/go/main/App";
import type { main } from "../../wailsjs/go/models";

type PickerState = {
  contextId: string;
  contextKey: string;
  choices: main.EditorRepoChoice[];
};

type WorkspacePickerState = {
  contextId: string;
  contextKey: string;
  worktrees: main.EditorWorktree[];
};

const NO_REPO_MSG = "No linked repo with a local path. Add one in the Repositories tab.";

/**
 * Shared flow for opening the in-app editor from a context. Nothing is checked
 * out until a branch is chosen: 2+ linked branches -> a picker; exactly one ->
 * it is the choice; the chosen branch gets its worktree (on-checkout script runs
 * when it is created) and the editor opens there. No named branch but a linked
 * repo with a local path -> a repo -> branch checkout picker; nothing
 * checkout-able -> an error.
 */
export function useOpenContextEditor(openEditorTab: (contextId: string, contextKey: string, initialCwd?: string) => void) {
  const [picker, setPicker] = useState<PickerState | null>(null);
  const [workspacePicker, setWorkspacePicker] = useState<WorkspacePickerState | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const open = async (contextId: string, contextKey: string) => {
    setError(null);
    setLoading(true);
    try {
      const prep = await EnsureContextEditorWorkspace(contextId);
      if (prep.ready) {
        const worktrees = prep.worktrees ?? [];
        if (worktrees.length >= 2) {
          setWorkspacePicker({ contextId, contextKey, worktrees });
          return;
        }
        if (worktrees[0]) {
          const path = await EnsureBranchWorktree(worktrees[0].repoId, worktrees[0].branch);
          openEditorTab(contextId, contextKey, path);
        } else {
          openEditorTab(contextId, contextKey);
        }
        return;
      }
      if (prep.choices && prep.choices.length > 0) {
        setPicker({ contextId, contextKey, choices: prep.choices });
        return;
      }
      setError(NO_REPO_MSG);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  // Only the picked branch gets checked out (and its on-checkout script run).
  const confirmWorkspace = async (w: main.EditorWorktree) => {
    if (!workspacePicker) return;
    setError(null);
    setLoading(true);
    try {
      const path = await EnsureBranchWorktree(w.repoId, w.branch);
      openEditorTab(workspacePicker.contextId, workspacePicker.contextKey, path);
      setWorkspacePicker(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  const confirmPick = async (pathId: string, branch: string) => {
    if (!picker) return;
    setError(null);
    setLoading(true);
    try {
      await CheckoutInWorkspace(pathId, branch);
      openEditorTab(picker.contextId, picker.contextKey);
      setPicker(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  const cancel = () => {
    setPicker(null);
    setWorkspacePicker(null);
    setError(null);
  };

  return { open, picker, workspacePicker, confirmPick, confirmWorkspace, cancel, error, loading, setError };
}
