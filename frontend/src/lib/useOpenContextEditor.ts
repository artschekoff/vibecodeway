import { useState } from "react";
import { EnsureContextEditorWorkspace, CheckoutInWorkspace } from "../../wailsjs/go/main/App";
import type { main } from "../../wailsjs/go/models";

type PickerState = {
  contextId: string;
  contextKey: string;
  choices: main.EditorRepoChoice[];
};

const NO_REPO_MSG = "No linked repo with a local path. Add one in the Repositories tab.";

/**
 * Shared flow for opening the in-app editor from a context. Ensures a git
 * workspace exists first: if one is ready (or auto-created for a named branch)
 * the editor opens straight into it; otherwise it surfaces a repo → branch
 * checkout picker. Used by the "Open Editor" button and Solve with AI so both
 * land in the context's workspace, never the bare repo root.
 */
export function useOpenContextEditor(openEditorTab: (contextId: string, contextKey: string) => void) {
  const [picker, setPicker] = useState<PickerState | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const open = async (contextId: string, contextKey: string) => {
    setError(null);
    setLoading(true);
    try {
      const prep = await EnsureContextEditorWorkspace(contextId);
      if (prep.ready) {
        openEditorTab(contextId, contextKey);
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
    setError(null);
  };

  return { open, picker, confirmPick, cancel, error, loading, setError };
}
