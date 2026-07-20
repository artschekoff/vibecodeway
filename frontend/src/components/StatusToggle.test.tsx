import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import StatusToggle from "./StatusToggle";

describe("StatusToggle", () => {
  it("renders a checked checkbox when active", () => {
    render(<StatusToggle active={true} onToggle={vi.fn()} />);
    expect(screen.getByRole("checkbox")).toBeChecked();
  });

  it("renders an unchecked checkbox when not active", () => {
    render(<StatusToggle active={false} onToggle={vi.fn()} />);
    expect(screen.getByRole("checkbox")).not.toBeChecked();
  });

  it("adds the --on modifier class only when active", () => {
    const { container, rerender } = render(<StatusToggle active={true} onToggle={vi.fn()} />);
    expect(container.querySelector(".active-indicator--on")).toBeInTheDocument();
    rerender(<StatusToggle active={false} onToggle={vi.fn()} />);
    expect(container.querySelector(".active-indicator--on")).not.toBeInTheDocument();
  });

  it("calls onToggle when clicked", async () => {
    const onToggle = vi.fn();
    render(<StatusToggle active={true} onToggle={onToggle} />);
    await userEvent.click(screen.getByRole("checkbox"));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it("applies the title on the label", () => {
    render(<StatusToggle active={true} onToggle={vi.fn()} title="Archive this context" />);
    expect(screen.getByTitle("Archive this context")).toBeInTheDocument();
  });
});
