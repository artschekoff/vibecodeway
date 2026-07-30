import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { GetRepoFileTree } = vi.hoisted(() => ({ GetRepoFileTree: vi.fn() }));
vi.mock("../../../wailsjs/go/main/App", () => ({ GetRepoFileTree }));

import { FileTreePanel } from "./FileTreePanel";

beforeEach(() => {
  vi.clearAllMocks();
  GetRepoFileTree.mockResolvedValue(["src/app/main.ts", "src/app/util.ts", "README.md"]);
});

describe("FileTreePanel", () => {
  it("renders the tree (compacted folders) and opens a file with empty status", async () => {
    const onOpenFile = vi.fn();
    render(<FileTreePanel repoPath="/r/api" repoName="acme/api" onOpenFile={onOpenFile} />);
    // src/app compacts into one folder node; both files + README show.
    await waitFor(() => expect(screen.getByText("src/app/")).toBeInTheDocument());
    expect(screen.getByText("README.md")).toBeInTheDocument();
    expect(screen.getByText("main.ts")).toBeInTheDocument();

    fireEvent.click(screen.getByText("main.ts"));
    expect(onOpenFile).toHaveBeenCalledWith("/r/api", "acme/api", "src/app/main.ts", "");
  });

  it("collapses a folder to hide its files", async () => {
    render(<FileTreePanel repoPath="/r/api" repoName="acme/api" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("main.ts")).toBeInTheDocument());
    fireEvent.click(screen.getByText("src/app/"));
    expect(screen.queryByText("main.ts")).not.toBeInTheDocument();
    expect(screen.getByText("README.md")).toBeInTheDocument(); // sibling unaffected
  });
});
