import { useState } from "react";
import { EnsureContextEditorWorkspace, CheckoutInWorkspace } from "../../wailsjs/go/main/App";
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
 * Shared flow for opening the in-app editor from a context. Ensures a git
 * workspace exists first: 2+ ready worktrees -> a workspace picker; exactly one
 * -> open directly into it; none but a linked repo has a local path -> a repo ->
 * branch checkout picker; nothing checkout-able -> an error.
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
        openEditorTab(contextId, contextKey, worktrees[0]?.worktreePath);
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

  const confirmWorkspace = (worktreePath: string) => {
    if (!workspacePicker) return;
    openEditorTab(workspacePicker.contextId, workspacePicker.contextKey, worktreePath);
    setWorkspacePicker(null);
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
