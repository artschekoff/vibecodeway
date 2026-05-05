import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import UploadedFileCard from "./UploadedFileCard";
import type { UploadedFile } from "../types";

const sampleText = "This is the extracted text content of the uploaded file.\n".repeat(20).trim();

function makeFile(overrides: Partial<UploadedFile> = {}): UploadedFile {
  return {
    id: "uf-1",
    contextId: "ctx-1",
    originalName: "report.pdf",
    fileType: "pdf",
    extractedText: sampleText,
    fileSize: 52480,
    createdAt: "2025-06-15T10:30:00Z",
    updatedAt: "2025-06-15T10:30:00Z",
    isActive: true,
    ...overrides,
  };
}

describe("UploadedFileCard", () => {
  it("renders the file name", () => {
    render(<UploadedFileCard file={makeFile()} onDelete={vi.fn()} onToggle={vi.fn()} />);
    expect(screen.getByText("report.pdf")).toBeInTheDocument();
  });

  it("renders the file type badge", () => {
    render(<UploadedFileCard file={makeFile()} onDelete={vi.fn()} onToggle={vi.fn()} />);
    expect(screen.getByText("PDF")).toBeInTheDocument();
  });

  it("renders the file size", () => {
    render(<UploadedFileCard file={makeFile()} onDelete={vi.fn()} onToggle={vi.fn()} />);
    expect(screen.getByText(/51\.3\s*KB/)).toBeInTheDocument();
  });

  it("renders the created date", () => {
    render(<UploadedFileCard file={makeFile()} onDelete={vi.fn()} onToggle={vi.fn()} />);
    expect(screen.getByText(/Jun 15, 2025/)).toBeInTheDocument();
  });

  it("does not show extracted text when collapsed", () => {
    render(<UploadedFileCard file={makeFile()} onDelete={vi.fn()} onToggle={vi.fn()} />);
    const pre = document.querySelector(".uploaded-file-body");
    expect(pre).not.toBeInTheDocument();
  });

  it("shows full extracted text when expanded", async () => {
    const user = userEvent.setup();
    const file = makeFile();
    render(<UploadedFileCard file={file} onDelete={vi.fn()} onToggle={vi.fn()} />);

    const header = screen.getByText("report.pdf").closest("[class*='uploaded-file-header']")!;
    await user.click(header);

    const body = document.querySelector(".uploaded-file-body");
    expect(body).toBeInTheDocument();
    expect(body!.textContent).toContain("This is the extracted text content");
  });

  it("collapses back when clicked again", async () => {
    const user = userEvent.setup();
    render(<UploadedFileCard file={makeFile()} onDelete={vi.fn()} onToggle={vi.fn()} />);

    const header = screen.getByText("report.pdf").closest("[class*='uploaded-file-header']")!;
    await user.click(header);
    expect(document.querySelector(".uploaded-file-body")).toBeInTheDocument();

    await user.click(header);
    expect(document.querySelector(".uploaded-file-body")).not.toBeInTheDocument();
  });

  it("does not expand when extractedText is empty", async () => {
    const user = userEvent.setup();
    const file = makeFile({ extractedText: "" });
    render(<UploadedFileCard file={file} onDelete={vi.fn()} onToggle={vi.fn()} />);

    const header = screen.getByText("report.pdf").closest("[class*='uploaded-file-header']")!;
    await user.click(header);

    expect(document.querySelector(".uploaded-file-body")).not.toBeInTheDocument();
  });

  it("calls onDelete when delete button is clicked and confirmed", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    render(<UploadedFileCard file={makeFile()} onDelete={onDelete} onToggle={vi.fn()} />);

    const deleteBtn = screen.getByTitle("Remove uploaded file");
    await user.click(deleteBtn);

    // Confirm modal appears — click the confirm/Delete button
    const confirmBtn = screen.getByRole("button", { name: /delete/i });
    await user.click(confirmBtn);

    expect(onDelete).toHaveBeenCalledWith("uf-1");
  });

  it("calls onToggle when toggle is clicked", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    render(<UploadedFileCard file={makeFile()} onDelete={vi.fn()} onToggle={onToggle} />);

    const toggle = document.querySelector(".item-active-toggle input")!;
    await user.click(toggle);

    expect(onToggle).toHaveBeenCalledWith("uf-1");
  });

  it("applies inactive styling when file is not active", () => {
    const file = makeFile({ isActive: false });
    const { container } = render(<UploadedFileCard file={file} onDelete={vi.fn()} onToggle={vi.fn()} />);

    expect(container.firstChild).toHaveClass("uploaded-file-card--inactive");
  });

  it("does not apply inactive styling when file is active", () => {
    const file = makeFile({ isActive: true });
    const { container } = render(<UploadedFileCard file={file} onDelete={vi.fn()} onToggle={vi.fn()} />);

    expect(container.firstChild).not.toHaveClass("uploaded-file-card--inactive");
  });

  it("shows correct icon for docx file type", () => {
    const file = makeFile({ fileType: "docx", originalName: "doc.docx" });
    render(<UploadedFileCard file={file} onDelete={vi.fn()} onToggle={vi.fn()} />);
    const icon = document.querySelector(".uploaded-file-icon");
    expect(icon).toBeInTheDocument();
  });

  it("shows correct icon for txt file type", () => {
    const file = makeFile({ fileType: "txt", originalName: "notes.txt" });
    render(<UploadedFileCard file={file} onDelete={vi.fn()} onToggle={vi.fn()} />);
    const icon = document.querySelector(".uploaded-file-icon");
    expect(icon).toBeInTheDocument();
  });

  it("does not show expand chevron when extractedText is empty", () => {
    const file = makeFile({ extractedText: "" });
    render(<UploadedFileCard file={file} onDelete={vi.fn()} onToggle={vi.fn()} />);
    expect(document.querySelector(".expand-icon")).not.toBeInTheDocument();
  });

  it("shows expand chevron when extractedText is present", () => {
    render(<UploadedFileCard file={makeFile()} onDelete={vi.fn()} onToggle={vi.fn()} />);
    expect(document.querySelector(".expand-icon")).toBeInTheDocument();
  });
});
