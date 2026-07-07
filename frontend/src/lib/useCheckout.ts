import { useEffect, useState } from "react";
import {
  CheckoutInWorkspace,
  FindWorkspaceForBranch,
  ListAllWorkspaces,
  ListRepoLocalPaths,
  OpenWorkspaceTerminal,
} from "../../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import type { VCSRepoPath, VCSWorkspace } from "../types";

export type CheckoutLogLine = { line: string; stream: "stdout" | "stderr" };

export function useCheckout(
  onWorkspacesUpdated: (ws: VCSWorkspace[]) => void,
  noPathError = "No local path set for this repository."
) {
  const [checkoutTarget, setCheckoutTarget] = useState<{ branch: string; repoId: string } | null>(null);
  const [checkoutPaths, setCheckoutPaths] = useState<VCSRepoPath[]>([]);
  const [checkoutLoading, setCheckoutLoading] = useState(false);
  const [checkoutError, setCheckoutError] = useState("");

  const [logs, setLogs] = useState<CheckoutLogLine[]>([]);
  const [logRunning, setLogRunning] = useState(false);
  const [logExitCode, setLogExitCode] = useState<number | null>(null);
  const [logCommand, setLogCommand] = useState("");
  const [logOpen, setLogOpen] = useState(false);

  useEffect(() => {
    const offStart = EventsOn("checkout:start", (p: { path: string; cmd: string }) => {
      setLogs([]);
      setLogExitCode(null);
      setLogCommand(p?.cmd ?? "");
      setLogRunning(true);
      setLogOpen(true);
    });
    const offLog = EventsOn("checkout:log", (p: CheckoutLogLine) => {
      if (!p) return;
      setLogs(prev => [...prev, p]);
    });
    const offDone = EventsOn("checkout:done", (p: { exitCode: number; error: string }) => {
      setLogRunning(false);
      setLogExitCode(p?.exitCode ?? 0);
    });
    return () => {
      EventsOff("checkout:start");
      EventsOff("checkout:log");
      EventsOff("checkout:done");
      // Silence unused-var lint on the returned unsubscribers.
      void offStart; void offLog; void offDone;
    };
  }, []);

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

  const dismissLog = () => {
    setLogOpen(false);
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
    logs,
    logRunning,
    logExitCode,
    logCommand,
    logOpen,
    dismissLog,
  };
}
