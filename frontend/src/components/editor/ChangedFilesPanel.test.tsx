import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { GetContextChanges } = vi.hoisted(() => ({ GetContextChanges: vi.fn() }));
vi.mock("../../../wailsjs/go/main/App", () => ({ GetContextChanges }));

import { ChangedFilesPanel } from "./ChangedFilesPanel";

beforeEach(() => {
  vi.clearAllMocks();
  GetContextChanges.mockResolvedValue([
    { repoName: "acme/api", repoPath: "/r/api", files: [{ path: "src/a.ts", status: "modified", diff: "" }] },
    { repoName: "acme/db", repoPath: "/r/db", files: [{ path: "schema.sql", status: "added", diff: "" }] },
  ]);
});

describe("ChangedFilesPanel", () => {
  it("renders files grouped by repo and calls onOpenFile on click", async () => {
    const onOpenFile = vi.fn();
    render(<ChangedFilesPanel contextId="c1" onOpenFile={onOpenFile} />);

    await waitFor(() => expect(screen.getByText("acme/api")).toBeInTheDocument());
    expect(screen.getByText("acme/db")).toBeInTheDocument();

    fireEvent.click(screen.getByText("src/a.ts"));
    expect(onOpenFile).toHaveBeenCalledWith("/r/api", "acme/api", "src/a.ts", "modified");
  });
});
