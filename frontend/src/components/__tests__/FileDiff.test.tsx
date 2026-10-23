import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

const { GetFileDiff } = vi.hoisted(() => ({ GetFileDiff: vi.fn() }));
vi.mock("../../../wailsjs/go/main/App", () => ({ GetFileDiff }));

import FileDiff from "../FileDiff";

beforeEach(() => vi.clearAllMocks());

describe("FileDiff", () => {
  it("renders added and removed lines with distinct styling", async () => {
    const unified = [
      "diff --git a/a.txt b/a.txt",
      "--- a/a.txt",
      "+++ b/a.txt",
      "@@ -1 +1,2 @@",
      " one",
      "+two",
    ].join("\n");
    GetFileDiff.mockResolvedValue(unified);
    render(<FileDiff repoPath="/r" path="a.txt" status="modified" mode="head" />);
    expect(screen.getByText("a.txt")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { expanded: false }));
    const added = await waitFor(() => screen.getByText("+two"));
    expect(added.className).toMatch(/added|add/);
  });
});
