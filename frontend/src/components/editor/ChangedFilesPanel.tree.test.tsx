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
// Sections default to collapsed; seed the section (not the folder tree) open so
// these tests can focus on folder-tree behavior.
const seedExpanded = () =>
  localStorage.setItem(EXPAND_KEY, JSON.stringify(["acme/api:local"]));

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
        // A single-child folder chain (src -> functions -> cf-metric) should
        // compact onto one folder row, VS Code-style.
        { path: "src/functions/cf-metric/index.ts", status: "modified", diff: "", indexStatus: "", workStatus: "modified" },
        { path: "src/functions/cf-metric/handler.ts", status: "modified", diff: "", indexStatus: "", workStatus: "modified" },
        // A top-level flat file, unaffected by the nested tree.
        { path: "readme.md", status: "modified", diff: "", indexStatus: "", workStatus: "modified" },
      ],
    },
  ]);
});

describe("ChangedFilesPanel — folder tree", () => {
  it("compacts a single-child folder chain onto one row and shows file basenames", async () => {
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("src/functions/cf-metric")).toBeInTheDocument());
    // File rows show the basename, not the full path.
    expect(screen.getByText("index.ts")).toBeInTheDocument();
    expect(screen.getByText("handler.ts")).toBeInTheDocument();
    // The flat top-level file renders directly, without a folder row.
    expect(screen.getByText("readme.md")).toBeInTheDocument();
    expect(screen.queryByText("src/functions/cf-metric/index.ts")).not.toBeInTheDocument();
  });

  it("opens a nested file with its full path when clicked", async () => {
    const onOpenFile = vi.fn();
    render(<ChangedFilesPanel contextId="c1" onOpenFile={onOpenFile} />);
    await waitFor(() => expect(screen.getByText("index.ts")).toBeInTheDocument());
    fireEvent.click(screen.getByText("index.ts"));
    expect(onOpenFile).toHaveBeenCalledWith("/r/api", "acme/api", "src/functions/cf-metric/index.ts", "modified", "head");
  });

  it("collapses and expands a folder row on click", async () => {
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("index.ts")).toBeInTheDocument());

    fireEvent.click(screen.getByText("src/functions/cf-metric"));
    expect(screen.queryByText("index.ts")).not.toBeInTheDocument();
    expect(screen.queryByText("handler.ts")).not.toBeInTheDocument();
    // The unrelated flat file stays visible.
    expect(screen.getByText("readme.md")).toBeInTheDocument();

    fireEvent.click(screen.getByText("src/functions/cf-metric"));
    expect(screen.getByText("index.ts")).toBeInTheDocument();
  });

  it("collapses and expands every folder in a section via its collapse-all/expand-all controls", async () => {
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("index.ts")).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText("collapse all folders in Local Changes"));
    expect(screen.queryByText("index.ts")).not.toBeInTheDocument();
    expect(screen.queryByText("handler.ts")).not.toBeInTheDocument();
    // The folder row itself, and unrelated flat files, remain visible.
    expect(screen.getByText("src/functions/cf-metric")).toBeInTheDocument();
    expect(screen.getByText("readme.md")).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("expand all folders in Local Changes"));
    expect(screen.getByText("index.ts")).toBeInTheDocument();
    expect(screen.getByText("handler.ts")).toBeInTheDocument();
  });

  it("shows matching nested files under their folder while filtering", async () => {
    render(<ChangedFilesPanel contextId="c1" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("index.ts")).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText("filter changed files"), { target: { value: "handler" } });
    expect(screen.getByText("src/functions/cf-metric")).toBeInTheDocument();
    expect(screen.getByText("handler.ts")).toBeInTheDocument();
    expect(screen.queryByText("index.ts")).not.toBeInTheDocument();
    expect(screen.queryByText("readme.md")).not.toBeInTheDocument();
  });
});
