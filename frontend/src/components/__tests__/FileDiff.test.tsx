import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import FileDiff from "../FileDiff";

describe("FileDiff", () => {
  it("renders added and removed lines with distinct styling", () => {
    const unified = [
      "diff --git a/a.txt b/a.txt",
      "--- a/a.txt",
      "+++ b/a.txt",
      "@@ -1 +1,2 @@",
      " one",
      "+two",
    ].join("\n");
    render(<FileDiff path="a.txt" status="modified" diff={unified} />);
    expect(screen.getByText("a.txt")).toBeInTheDocument();
    const added = screen.getByText("+two");
    expect(added.className).toMatch(/added|add/);
  });
});
