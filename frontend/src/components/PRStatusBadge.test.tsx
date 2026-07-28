import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { PRStatusBadge } from "./PRStatusBadge";

describe("PRStatusBadge", () => {
  it("renders state, draft, approval and conflict badges", () => {
    render(<PRStatusBadge status="open" reviewStatus="approved" mergeable="CONFLICTING" ciStatus="FAILURE" isDraft />);
    expect(screen.getByText("Draft")).toBeInTheDocument();
    expect(screen.getByText("open")).toBeInTheDocument();
    expect(screen.getByText("Approved")).toBeInTheDocument();
    expect(screen.getByText("Conflicts")).toBeInTheDocument();
    expect(screen.getByText("CI failed")).toBeInTheDocument();
  });

  it("hides review/CI badges for a merged PR", () => {
    render(<PRStatusBadge status="merged" reviewStatus="approved" ciStatus="FAILURE" />);
    expect(screen.getByText("merged")).toBeInTheDocument();
    expect(screen.queryByText("Approved")).not.toBeInTheDocument();
    expect(screen.queryByText("CI failed")).not.toBeInTheDocument();
  });
});
