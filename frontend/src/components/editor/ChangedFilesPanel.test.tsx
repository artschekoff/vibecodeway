import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { GetContextChanges, StageFile, UnstageFile, DiscardFile, ConfirmAction } = vi.hoisted(() => ({
  GetContextChanges: vi.fn(),
  StageFile: vi.fn(),
  UnstageFile: vi.fn(),
  DiscardFile: vi.fn(),
  ConfirmAction: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({ GetContextChanges, StageFile, UnstageFile, DiscardFile, ConfirmAction }));

import { ChangedFilesPanel } from "./ChangedFilesPanel";

const EXPAND_KEY = "vibecodeway:changedfiles:expanded";
// Sections are collapsed by default; most tests want them open, so seed every
// section key as expanded. Tests exercising the default/collapse behavior clear it.
const seedExpanded = () =>
  localStorage.setItem(EXPAND_KEY, JSON.stringify([
    "acme/api:branch", "acme/api:staged", "acme/api:local",
    "acme/rdbms:branch", "acme/rdbms:staged", "acme/rdbms:local",
  ]));

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  seedExpanded();
  StageFile.mockResolvedValue(undefined);
  UnstageFile.mockResolvedValue(undefined);
  DiscardFile.mockResolvedValue(undefined);
  ConfirmAction.mockResolvedValue(true);
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
  it("warns when the status limit truncated the list (VS Code git.statusLimit)", async () => {
    GetContextChanges.mockResolvedValue([
      {
        repoName: "acme/api",
        repoPath: "/r/api",
        didHitLimit: true,
        statusLimit: 10000,
        files: [{ path: "work.ts", status: "modified", diff: "", indexStatus: "", workStatus: "modified" }],
      },
    ]);
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent(/only the first 10,000/));
  });

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
    expect(onOpenFile).toHaveBeenCalledWith("/r/api", "acme/api", "work.ts", "modified", "head");
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
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("work.ts")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("discard work.ts"));
    await waitFor(() => expect(DiscardFile).toHaveBeenCalledWith("/r/api", "work.ts"));
  });

  it("does not discard when the confirmation is declined", async () => {
    ConfirmAction.mockResolvedValue(false);
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("work.ts")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("discard work.ts"));
    await waitFor(() => expect(ConfirmAction).toHaveBeenCalled());
    expect(DiscardFile).not.toHaveBeenCalled();
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
    expect(onOpenFile).toHaveBeenCalledWith("/r/api", "acme/api", "feature.ts", "added", "branch");
  });

  it("has no per-repo refresh button — the sidebar reload owns re-sync", async () => {
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("acme/api")).toBeInTheDocument());
    expect(screen.queryByLabelText("refresh from github")).not.toBeInTheDocument();
  });

  it("copies the repo path and opens a terminal from the repo header", async () => {
    const onOpenTerminal = vi.fn();
    const writeText = vi.fn();
    Object.assign(navigator, { clipboard: { writeText } });
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} onOpenTerminal={onOpenTerminal} />);
    await waitFor(() => expect(screen.getByText("acme/api")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("copy path for acme/api"));
    expect(writeText).toHaveBeenCalledWith("/r/api");
    fireEvent.click(screen.getByLabelText("open terminal in acme/api"));
    expect(onOpenTerminal).toHaveBeenCalledWith("/r/api");
  });

  it("collapses sections by default; expand-all / collapse-all toggles them", async () => {
    localStorage.clear(); // no seed → default collapsed
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    // Section headers render, but their files stay hidden while collapsed.
    await waitFor(() => expect(screen.getByText("Local Changes")).toBeInTheDocument());
    expect(screen.queryByText("work.ts")).not.toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("expand all"));
    expect(screen.getByText("work.ts")).toBeInTheDocument();
    expect(screen.getByText("staged.ts")).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("collapse all"));
    expect(screen.queryByText("work.ts")).not.toBeInTheDocument();
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
