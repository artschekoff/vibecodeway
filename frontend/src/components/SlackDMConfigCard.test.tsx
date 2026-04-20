import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import SlackDMConfigCard from "./SlackDMConfigCard";
import type { SlackDMConfig } from "../types";

const longContent = "Line 1: This is a long message about the project.\n".repeat(10).trim();

function makeDM(overrides: Partial<SlackDMConfig> = {}): SlackDMConfig {
  return {
    id: "dm-1",
    contextId: "ctx-1",
    workspace: "acme",
    startFrom: "2025-01-01",
    content: longContent,
    createdAt: "2025-01-01T00:00:00Z",
    updatedAt: "2025-01-01T00:00:00Z",
    isActive: true,
    ...overrides,
  };
}

function findPreContent(): string | null {
  const pre = document.querySelector(".slack-thread-content");
  return pre?.textContent ?? null;
}

describe("SlackDMConfigCard", () => {
  it("does not show full content in collapsed state", () => {
    const dm = makeDM();
    render(<SlackDMConfigCard dm={dm} onDelete={vi.fn()} onToggle={vi.fn()} />);

    expect(findPreContent()).toBeNull();
  });

  it("shows full content when expanded", async () => {
    const user = userEvent.setup();
    const dm = makeDM();
    render(<SlackDMConfigCard dm={dm} onDelete={vi.fn()} onToggle={vi.fn()} />);

    const header = screen.getByText(/DMs · acme/).closest("[class*='issue-card-header']")!;
    await user.click(header);

    expect(findPreContent()).toBe(longContent);
  });

  it("does not expand when there is no content", async () => {
    const user = userEvent.setup();
    const dm = makeDM({ content: "" });
    render(<SlackDMConfigCard dm={dm} onDelete={vi.fn()} onToggle={vi.fn()} />);

    const header = screen.getByText(/DMs · acme/).closest("[class*='issue-card-header']")!;
    await user.click(header);

    expect(findPreContent()).toBeNull();
  });

  it("collapses back when clicked again", async () => {
    const user = userEvent.setup();
    const dm = makeDM();
    render(<SlackDMConfigCard dm={dm} onDelete={vi.fn()} onToggle={vi.fn()} />);

    const header = screen.getByText(/DMs · acme/).closest("[class*='issue-card-header']")!;
    await user.click(header);
    expect(findPreContent()).toBe(longContent);

    await user.click(header);
    expect(findPreContent()).toBeNull();
  });

  it("renders JSON DM content as structured messages", async () => {
    const user = userEvent.setup();
    const jsonContent = JSON.stringify([
      {
        id: "D1",
        userId: "U1",
        userName: "alice",
        isIm: true,
        messages: [
          { user: "alice", text: "hello there", timestamp: "2025-01-02 10:00" },
          { user: "bob", text: "hi alice", timestamp: "2025-01-02 10:05" },
        ],
      },
    ]);
    const dm = makeDM({ content: jsonContent });
    render(<SlackDMConfigCard dm={dm} onDelete={vi.fn()} onToggle={vi.fn()} />);

    const header = screen.getByText(/DMs · acme/).closest("[class*='issue-card-header']")!;
    await user.click(header);

    expect(findPreContent()).toBeNull();
    expect(document.querySelector(".dm-conversations")).toBeInTheDocument();
    expect(document.querySelector(".dm-conversation-header")?.textContent).toBe("alice");
    expect(screen.getByText("hello there")).toBeInTheDocument();
    expect(screen.getByText("hi alice")).toBeInTheDocument();
    expect(screen.getByText("2025-01-02 10:00")).toBeInTheDocument();
  });

  it("shows 'no messages' for DM entries without messages alongside ones that have them", async () => {
    const user = userEvent.setup();
    const jsonContent = JSON.stringify([
      { id: "D1", userName: "alice", messages: [] },
      { id: "D2", userName: "bob", messages: [{ user: "bob", text: "hey", timestamp: "2025-01-03 09:00" }] },
    ]);
    const dm = makeDM({ content: jsonContent });
    render(<SlackDMConfigCard dm={dm} onDelete={vi.fn()} onToggle={vi.fn()} />);

    const header = screen.getByText(/DMs · acme/).closest("[class*='issue-card-header']")!;
    await user.click(header);

    const headers = document.querySelectorAll(".dm-conversation-header");
    expect(headers).toHaveLength(2);
    expect(headers[0].textContent).toBe("alice");
    expect(headers[1].textContent).toBe("bob");
    expect(screen.getByText("No messages in time range")).toBeInTheDocument();
    expect(screen.getByText("hey")).toBeInTheDocument();
  });

  it("renders raw list-dms entries (no messages field) with group names", async () => {
    const user = userEvent.setup();
    const jsonContent = JSON.stringify([
      { id: "C0AMJ", name: "mpdm-oleksiy--liora--artem-1", isIm: false },
      { id: "D0ATW", userId: "U0AU2", userName: "Maksim Vaitkiavichus", isIm: true },
    ]);
    const dm = makeDM({ content: jsonContent });
    render(<SlackDMConfigCard dm={dm} onDelete={vi.fn()} onToggle={vi.fn()} />);

    const header = screen.getByText(/DMs · acme/).closest("[class*='issue-card-header']")!;
    await user.click(header);

    expect(document.querySelector(".dm-conversations")).toBeInTheDocument();
    const headers = document.querySelectorAll(".dm-conversation-header");
    expect(headers).toHaveLength(2);
    expect(headers[0].textContent).toBe("mpdm-oleksiy--liora--artem-1");
    expect(headers[1].textContent).toBe("Maksim Vaitkiavichus");
    expect(screen.getAllByText("No messages in time range")).toHaveLength(2);
  });

  it("shows a delete button for each DM entry when onDeleteEntry is provided", async () => {
    const user = userEvent.setup();
    const jsonContent = JSON.stringify([
      { id: "D1", userName: "alice", messages: [{ user: "alice", text: "hi", timestamp: "2025-01-02 10:00" }] },
      { id: "D2", userName: "bob", messages: [] },
    ]);
    const dm = makeDM({ content: jsonContent });
    const onDeleteEntry = vi.fn();
    render(<SlackDMConfigCard dm={dm} onDelete={vi.fn()} onToggle={vi.fn()} onDeleteEntry={onDeleteEntry} />);

    const header = screen.getByText(/DMs · acme/).closest("[class*='issue-card-header']")!;
    await user.click(header);

    const deleteButtons = document.querySelectorAll(".dm-entry-delete-btn");
    expect(deleteButtons).toHaveLength(2);
  });

  it("calls onDeleteEntry with dmId and entryId when delete button is clicked", async () => {
    const user = userEvent.setup();
    const jsonContent = JSON.stringify([
      { id: "D1", userName: "alice", messages: [] },
      { id: "D2", userName: "bob", messages: [] },
    ]);
    const dm = makeDM({ content: jsonContent });
    const onDeleteEntry = vi.fn();
    render(<SlackDMConfigCard dm={dm} onDelete={vi.fn()} onToggle={vi.fn()} onDeleteEntry={onDeleteEntry} />);

    const header = screen.getByText(/DMs · acme/).closest("[class*='issue-card-header']")!;
    await user.click(header);

    const deleteButtons = document.querySelectorAll(".dm-entry-delete-btn");
    await user.click(deleteButtons[0]);

    expect(onDeleteEntry).toHaveBeenCalledTimes(1);
    expect(onDeleteEntry).toHaveBeenCalledWith("dm-1", "D1");
  });

  it("does not render entry delete buttons when onDeleteEntry is not provided", async () => {
    const user = userEvent.setup();
    const jsonContent = JSON.stringify([
      { id: "D1", userName: "alice", messages: [] },
    ]);
    const dm = makeDM({ content: jsonContent });
    render(<SlackDMConfigCard dm={dm} onDelete={vi.fn()} onToggle={vi.fn()} />);

    const header = screen.getByText(/DMs · acme/).closest("[class*='issue-card-header']")!;
    await user.click(header);

    expect(document.querySelector(".dm-entry-delete-btn")).not.toBeInTheDocument();
  });
});
