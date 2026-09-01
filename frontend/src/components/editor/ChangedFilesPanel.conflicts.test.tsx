import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { GetContextChanges, StageFile, UnstageFile, DiscardFile, ConfirmAction, ResolveConflict } =
  vi.hoisted(() => ({
    GetContextChanges: vi.fn(), StageFile: vi.fn(), UnstageFile: vi.fn(),
    DiscardFile: vi.fn(), ConfirmAction: vi.fn(), ResolveConflict: vi.fn(),
  }));
vi.mock("../../../wailsjs/go/main/App", () => ({
  GetContextChanges, StageFile, UnstageFile, DiscardFile, ConfirmAction, ResolveConflict,
}));

import { ChangedFilesPanel } from "./ChangedFilesPanel";

const EXPAND_KEY = "vibecodeway:changedfiles:expanded";

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  localStorage.setItem(EXPAND_KEY, JSON.stringify([
    "acme/api:branch", "acme/api:staged", "acme/api:local", "acme/api:conflicts",
  ]));
  StageFile.mockResolvedValue(undefined);
  ResolveConflict.mockResolvedValue(undefined);
  ConfirmAction.mockResolvedValue(true);
  GetContextChanges.mockResolvedValue([
    {
      repoName: "acme/api", repoPath: "/r/api",
      files: [
        { path: "both.ts", status: "conflict", diff: "", indexStatus: "", workStatus: "", conflicted: true },
        { path: "work.ts", status: "modified", diff: "", indexStatus: "", workStatus: "modified" },
      ],
    },
  ]);
});

describe("ChangedFilesPanel — merge conflicts", () => {
  it("lists conflicts in their own section, above the ordinary changes", async () => {
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Merge Conflicts")).toBeInTheDocument());
    expect(screen.getByText("both.ts")).toBeInTheDocument();
  });

  it("takes ours or theirs for a whole file", async () => {
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("both.ts")).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText("keep ours for both.ts"));
    await waitFor(() => expect(ResolveConflict).toHaveBeenCalledWith("/r/api", "both.ts", "ours"));

    fireEvent.click(screen.getByLabelText("keep theirs for both.ts"));
    await waitFor(() => expect(ResolveConflict).toHaveBeenCalledWith("/r/api", "both.ts", "theirs"));
  });

  it("marks a hand-edited file resolved by staging it", async () => {
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("both.ts")).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText("mark both.ts resolved"));
    await waitFor(() => expect(StageFile).toHaveBeenCalledWith("/r/api", "both.ts"));
  });

  it("opens a conflicted file so the markers can be edited by hand", async () => {
    const onOpenFile = vi.fn();
    render(<ChangedFilesPanel contextId="c1" onOpenFile={onOpenFile} />);
    await waitFor(() => expect(screen.getByText("both.ts")).toBeInTheDocument());

    fireEvent.click(screen.getByText("both.ts"));
    expect(onOpenFile).toHaveBeenCalledWith("/r/api", "acme/api", "both.ts", "conflict", "head");
  });

  it("shows no conflicts section when nothing is conflicted", async () => {
    GetContextChanges.mockResolvedValue([{
      repoName: "acme/api", repoPath: "/r/api",
      files: [{ path: "work.ts", status: "modified", diff: "", indexStatus: "", workStatus: "modified" }],
    }]);
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("work.ts")).toBeInTheDocument());
    expect(screen.queryByText("Merge Conflicts")).not.toBeInTheDocument();
  });
});
