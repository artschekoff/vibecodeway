import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { GetContextChanges, StageFile, UnstageFile, DiscardFile } = vi.hoisted(() => ({
  GetContextChanges: vi.fn(),
  StageFile: vi.fn(),
  UnstageFile: vi.fn(),
  DiscardFile: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({ GetContextChanges, StageFile, UnstageFile, DiscardFile }));

import { ChangedFilesPanel } from "./ChangedFilesPanel";

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  StageFile.mockResolvedValue(undefined);
  UnstageFile.mockResolvedValue(undefined);
  DiscardFile.mockResolvedValue(undefined);
  GetContextChanges.mockResolvedValue([
    {
      repoName: "acme/api",
      repoPath: "/r/api",
      files: [
        { path: "staged.ts", status: "modified", diff: "", indexStatus: "modified", workStatus: "" },
        { path: "work.ts", status: "modified", diff: "", indexStatus: "", workStatus: "modified" },
      ],
    },
  ]);
});

describe("ChangedFilesPanel", () => {
  it("groups files into Staged Changes and Local Changes", async () => {
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Staged Changes")).toBeInTheDocument());
    expect(screen.getByText("Local Changes")).toBeInTheDocument();
    expect(screen.getByText("staged.ts")).toBeInTheDocument();
    expect(screen.getByText("work.ts")).toBeInTheDocument();
  });

  it("shows a file that is both in the PR and edited locally in both groups", async () => {
    GetContextChanges.mockResolvedValue([
      {
        repoName: "acme/api",
        repoPath: "/r/api",
        files: [
          // Same path: one branch (PR) entry + one local working-tree entry.
          { path: "a.ts", status: "modified", diff: "", indexStatus: "", workStatus: "", commitStatus: "modified" },
          { path: "a.ts", status: "modified", diff: "", indexStatus: "", workStatus: "modified", commitStatus: "" },
        ],
      },
    ]);
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Branch Changes")).toBeInTheDocument());
    expect(screen.getByText("Local Changes")).toBeInTheDocument();
    // a.ts appears once under each group.
    expect(screen.getAllByText("a.ts")).toHaveLength(2);
  });

  it("shows only the active repo's files when activeRepoPath is set", async () => {
    GetContextChanges.mockResolvedValue([
      { repoName: "acme/api", repoPath: "/r/api", files: [{ path: "api.ts", status: "modified", diff: "", indexStatus: "", workStatus: "modified" }] },
      { repoName: "acme/rdbms", repoPath: "/r/rdbms", files: [{ path: "db.ts", status: "modified", diff: "", indexStatus: "", workStatus: "modified" }] },
    ]);
    render(<ChangedFilesPanel contextId="c1" activeRepoPath="/r/rdbms" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("db.ts")).toBeInTheDocument());
    expect(screen.queryByText("api.ts")).not.toBeInTheDocument();
    expect(screen.queryByText("acme/api")).not.toBeInTheDocument();
  });

  it("collapses and expands a section when its header is clicked", async () => {
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("work.ts")).toBeInTheDocument());

    // Collapse "Local Changes" — its files disappear.
    fireEvent.click(screen.getByText("Local Changes"));
    expect(screen.queryByText("work.ts")).not.toBeInTheDocument();
    // Other sections are unaffected.
    expect(screen.getByText("staged.ts")).toBeInTheDocument();

    // Expand again — files return.
    fireEvent.click(screen.getByText("Local Changes"));
    expect(screen.getByText("work.ts")).toBeInTheDocument();
  });

  it("opens the diff when a filename is clicked", async () => {
    const onOpenFile = vi.fn();
    render(<ChangedFilesPanel contextId="c1" onOpenFile={onOpenFile} />);
    await waitFor(() => expect(screen.getByText("work.ts")).toBeInTheDocument());
    fireEvent.click(screen.getByText("work.ts"));
    expect(onOpenFile).toHaveBeenCalledWith("/r/api", "acme/api", "work.ts", "modified");
  });

  it("stages an unstaged file", async () => {
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("work.ts")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("stage work.ts"));
    await waitFor(() => expect(StageFile).toHaveBeenCalledWith("/r/api", "work.ts"));
  });

  it("unstages a staged file", async () => {
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("staged.ts")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("unstage staged.ts"));
    await waitFor(() => expect(UnstageFile).toHaveBeenCalledWith("/r/api", "staged.ts"));
  });

  it("discards an unstaged file after confirmation", async () => {
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(true);
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("work.ts")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("discard work.ts"));
    await waitFor(() => expect(DiscardFile).toHaveBeenCalledWith("/r/api", "work.ts"));
    confirmSpy.mockRestore();
  });

  it("shows committed branch changes read-only (no stage/discard)", async () => {
    GetContextChanges.mockResolvedValue([
      {
        repoName: "acme/api",
        repoPath: "/r/api",
        files: [
          { path: "feature.ts", status: "added", diff: "", indexStatus: "", workStatus: "", commitStatus: "added" },
        ],
      },
    ]);
    const onOpenFile = vi.fn();
    render(<ChangedFilesPanel contextId="c1" onOpenFile={onOpenFile} />);
    await waitFor(() => expect(screen.getByText("Branch Changes")).toBeInTheDocument());
    // Read-only: no stage/discard actions for a committed-only file.
    expect(screen.queryByLabelText("stage feature.ts")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("discard feature.ts")).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("feature.ts"));
    expect(onOpenFile).toHaveBeenCalledWith("/r/api", "acme/api", "feature.ts", "added");
  });

  it("filters files by name", async () => {
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("staged.ts")).toBeInTheDocument());
    expect(screen.getByText("work.ts")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("filter changed files"), { target: { value: "staged" } });
    expect(screen.getByText("staged.ts")).toBeInTheDocument();
    expect(screen.queryByText("work.ts")).not.toBeInTheDocument();
  });
});
