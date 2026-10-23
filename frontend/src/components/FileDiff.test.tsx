import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

const { GetFileDiff } = vi.hoisted(() => ({ GetFileDiff: vi.fn() }));
vi.mock("../../wailsjs/go/main/App", () => ({ GetFileDiff }));

import FileDiff from "./FileDiff";

const diff = "diff --git a/x b/x\n@@ -0,0 +1,2 @@\n+one\n+two\n-old";

beforeEach(() => {
  vi.clearAllMocks();
  GetFileDiff.mockResolvedValue(diff);
});

describe("FileDiff", () => {
  it("starts collapsed without fetching; expand loads the diff and shows +/− counts", async () => {
    render(<FileDiff repoPath="/r" path="x.ts" status="M" mode="head" />);
    expect(GetFileDiff).not.toHaveBeenCalled();
    expect(screen.queryByText("+one")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { expanded: false }));
    await waitFor(() => expect(screen.getByText("+one")).toBeInTheDocument());
    expect(GetFileDiff).toHaveBeenCalledWith("/r", "x.ts", "head");
    expect(screen.getByText("+2")).toBeInTheDocument();
    expect(screen.getByText("−1")).toBeInTheDocument();
  });

  it("follows the expandAll signal", async () => {
    const { rerender } = render(<FileDiff repoPath="/r" path="x.ts" status="M" mode="head" expandAll={false} />);
    expect(screen.queryByText("+one")).not.toBeInTheDocument();
    rerender(<FileDiff repoPath="/r" path="x.ts" status="M" mode="head" expandAll={true} />);
    await waitFor(() => expect(screen.getByText("+one")).toBeInTheDocument());
  });

  it("fetches once — collapsing and re-expanding reuses the loaded diff", async () => {
    render(<FileDiff repoPath="/r" path="x.ts" status="M" mode="head" />);
    fireEvent.click(screen.getByRole("button", { expanded: false }));
    await waitFor(() => expect(screen.getByText("+one")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { expanded: true }));
    fireEvent.click(screen.getByRole("button", { expanded: false }));
    await waitFor(() => expect(screen.getByText("+one")).toBeInTheDocument());
    expect(GetFileDiff).toHaveBeenCalledTimes(1);
  });
});
