import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { GetRepoFileTree } = vi.hoisted(() => ({ GetRepoFileTree: vi.fn() }));
vi.mock("../../../wailsjs/go/main/App", () => ({ GetRepoFileTree }));

import { FileTreePanel } from "./FileTreePanel";
import { HotkeyProvider } from "../../hotkeys/HotkeyProvider";

const renderTree = (ui: React.ReactElement) => render(<HotkeyProvider platform="darwin">{ui}</HotkeyProvider>);

beforeEach(() => {
  vi.clearAllMocks();
  GetRepoFileTree.mockResolvedValue(["src/app/main.ts", "src/app/util.ts", "README.md"]);
});

describe("FileTreePanel", () => {
  it("collapses folders by default; expand-all reveals files and a click opens one", async () => {
    const onOpenFile = vi.fn();
    renderTree(<FileTreePanel repoPath="/r/api" repoName="acme/api" onOpenFile={onOpenFile} />);
    // Folder row shows, but its nested files are hidden while collapsed.
    await waitFor(() => expect(screen.getByText("src/app/")).toBeInTheDocument());
    expect(screen.queryByText("main.ts")).not.toBeInTheDocument();
    // README is at the root (not inside a folder) so it's always visible.
    expect(screen.getByText("README.md")).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("expand all"));
    expect(screen.getByText("main.ts")).toBeInTheDocument();

    fireEvent.click(screen.getByText("main.ts"));
    expect(onOpenFile).toHaveBeenCalledWith("/r/api", "acme/api", "src/app/main.ts", "", "head");
  });

  it("collapse-all hides files again", async () => {
    renderTree(<FileTreePanel repoPath="/r/api" repoName="acme/api" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("src/app/")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("expand all"));
    expect(screen.getByText("main.ts")).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("collapse all"));
    expect(screen.queryByText("main.ts")).not.toBeInTheDocument();
  });

  it("opens a terminal at the repo path", async () => {
    const onOpenTerminal = vi.fn();
    renderTree(<FileTreePanel repoPath="/r/api" repoName="acme/api" onOpenFile={vi.fn()} onOpenTerminal={onOpenTerminal} />);
    await waitFor(() => expect(screen.getByText("src/app/")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("open terminal in acme/api"));
    expect(onOpenTerminal).toHaveBeenCalledWith("/r/api");
  });

  it("an active filter forces folders open so matches show", async () => {
    renderTree(<FileTreePanel repoPath="/r/api" repoName="acme/api" onOpenFile={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("src/app/")).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText("filter repo files"), { target: { value: "util" } });
    expect(screen.getByText("util.ts")).toBeInTheDocument();
    expect(screen.queryByText("main.ts")).not.toBeInTheDocument();
  });
});
