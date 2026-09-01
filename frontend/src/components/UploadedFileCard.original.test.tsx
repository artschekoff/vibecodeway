import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";

const H = vi.hoisted(() => ({ GetUploadedFileContent: vi.fn() }));
vi.mock("../../wailsjs/go/main/App", () => H);

import UploadedFileCard from "./UploadedFileCard";
import type { UploadedFile } from "../types";

function makeFile(over: Partial<UploadedFile> = {}): UploadedFile {
  return {
    id: "uf-1", contextId: "ctx-1", originalName: "report.pdf", fileType: "pdf",
    extractedText: "extracted words", fileSize: 1024,
    createdAt: "2026-08-20T10:30:00Z", updatedAt: "2026-08-20T10:30:00Z", isActive: true,
    ...over,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  H.GetUploadedFileContent.mockResolvedValue("QUJD"); // base64("ABC")
});

// The originals were stored all along; the card only ever showed the extracted
// text, so the picture the user attached was unreachable.
describe("UploadedFileCard — the original file", () => {
  it("shows the picture itself for an image, alongside the extracted text", async () => {
    render(<UploadedFileCard file={makeFile({ originalName: "shot.png", fileType: "png" })} />);
    await userEvent.click(screen.getByText("shot.png"));

    const img = await screen.findByAltText("shot.png");
    expect(img).toHaveAttribute("src", "data:image/png;base64,QUJD");
    // The text extraction is a companion to the original, not a replacement.
    expect(screen.getByText("extracted words")).toBeInTheDocument();
  });

  it("fetches the bytes once, not on every re-expand", async () => {
    render(<UploadedFileCard file={makeFile({ originalName: "shot.png", fileType: "png" })} />);
    await userEvent.click(screen.getByText("shot.png"));
    await screen.findByAltText("shot.png");
    await userEvent.click(screen.getByText("shot.png")); // collapse
    await userEvent.click(screen.getByText("shot.png")); // expand again
    await waitFor(() => expect(H.GetUploadedFileContent).toHaveBeenCalledTimes(1));
  });

  it("offers the original for download on a document, with no inline preview", async () => {
    render(<UploadedFileCard file={makeFile()} />);
    await userEvent.click(screen.getByText("report.pdf"));

    expect(screen.queryByAltText("report.pdf")).not.toBeInTheDocument();
    const save = await screen.findByRole("button", { name: "Save original" });

    const click = vi.fn();
    const created = vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:x");
    vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => {});
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(click);

    await userEvent.click(save);
    await waitFor(() => expect(click).toHaveBeenCalled());
    expect(created).toHaveBeenCalled();
  });

  it("says so when the original cannot be read, instead of failing silently", async () => {
    H.GetUploadedFileContent.mockRejectedValue(new Error("gone"));
    render(<UploadedFileCard file={makeFile({ originalName: "shot.png", fileType: "png" })} />);
    await userEvent.click(screen.getByText("shot.png"));
    expect(await screen.findByText(/original.*(unavailable|could not)/i)).toBeInTheDocument();
  });
});
