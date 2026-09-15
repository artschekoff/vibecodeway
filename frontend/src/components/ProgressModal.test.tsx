import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import ProgressModal from "./ProgressModal";
import * as progressCtx from "../contexts/ProgressContext";

const baseProgress = {
  isActive: true,
  title: "Refreshing Context",
  items: [],
  currentIndex: -1,
  current: 2,
  total: 4,
  stepText: "JH-5127",
  subStep: "Fetching pull requests",
  cancellable: true,
  cancel: vi.fn(),
};

describe("ProgressModal minimize", () => {
  it("collapses to a corner pill and restores on click", () => {
    vi.spyOn(progressCtx, "useProgress").mockReturnValue(baseProgress as never);
    render(<ProgressModal />);

    expect(screen.getByText("Refreshing Context")).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("minimize progress"));

    // Overlay gone, pill present with title + percent.
    expect(screen.queryByLabelText("minimize progress")).not.toBeInTheDocument();
    const pill = screen.getByTestId("progress-minimized");
    expect(pill).toHaveTextContent("Refreshing Context 50%");
    expect(pill).toHaveTextContent("Fetching pull requests");

    fireEvent.click(pill);
    expect(screen.getByLabelText("minimize progress")).toBeInTheDocument();
  });

  it("renders nothing when inactive", () => {
    vi.spyOn(progressCtx, "useProgress").mockReturnValue({ ...baseProgress, isActive: false } as never);
    const { container } = render(<ProgressModal />);
    expect(container).toBeEmptyDOMElement();
  });
});
