import { useState } from "react";
import {
  CheckoutInWorkspace,
  FindWorkspaceForBranch,
  ListAllWorkspaces,
  ListRepoLocalPaths,
  OpenWorkspaceTerminal,
} from "../../wailsjs/go/main/App";
import type { VCSRepoPath, VCSWorkspace } from "../types";

export function useCheckout(
  onWorkspacesUpdated: (ws: VCSWorkspace[]) => void,
  noPathError = "No local path set for this repository."
) {
  const [checkoutTarget, setCheckoutTarget] = useState<{ branch: string; repoId: string } | null>(null);
  const [checkoutPaths, setCheckoutPaths] = useState<VCSRepoPath[]>([]);
  const [checkoutLoading, setCheckoutLoading] = useState(false);
  const [checkoutError, setCheckoutError] = useState("");

  const refreshWorkspaces = async () => {
    const ws = (await ListAllWorkspaces() as VCSWorkspace[]) ?? [];
    onWorkspacesUpdated(ws);
  };

  const checkout = async (branch: string, repoId: string) => {
    setCheckoutError("");
    setCheckoutLoading(true);
    try {
      const existing = await FindWorkspaceForBranch(repoId, branch) as VCSWorkspace | null;
      if (existing) {
        await OpenWorkspaceTerminal(existing.id);
        return;
      }
      const paths = (await ListRepoLocalPaths(repoId) as VCSRepoPath[]) ?? [];
      if (paths.length === 0) {
        setCheckoutError(noPathError);
        return;
      }
      if (paths.length === 1) {
        await CheckoutInWorkspace(paths[0].id, branch);
        await refreshWorkspaces();
        return;
      }
      setCheckoutPaths(paths);
      setCheckoutTarget({ branch, repoId });
    } catch (e) {
      setCheckoutError(String(e));
    } finally {
      setCheckoutLoading(false);
    }
  };

  const checkoutWithPath = async (pathId: string) => {
    if (!checkoutTarget) return;
    setCheckoutError("");
    setCheckoutLoading(true);
    try {
      await CheckoutInWorkspace(pathId, checkoutTarget.branch);
      await refreshWorkspaces();
      setCheckoutTarget(null);
      setCheckoutPaths([]);
    } catch (e) {
      setCheckoutError(String(e));
    } finally {
      setCheckoutLoading(false);
    }
  };

  const dismissCheckoutModal = () => {
    setCheckoutTarget(null);
    setCheckoutPaths([]);
    setCheckoutError("");
  };

  return {
    checkout,
    checkoutWithPath,
    dismissCheckoutModal,
    checkoutTarget,
    checkoutPaths,
    checkoutLoading,
    checkoutError,
    setCheckoutError,
  };
}
