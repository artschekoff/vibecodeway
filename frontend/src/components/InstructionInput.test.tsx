import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { InstructionInput } from "./InstructionInput";
import type { MentionItem } from "../types/index";

vi.mock("../../wailsjs/go/main/App", () => ({
  SearchMessages: vi.fn().mockResolvedValue([]),
}));

const noop = async () => {};

const makeMentionItem = (overrides: Partial<MentionItem> = {}): MentionItem => ({
  id: "item-1",
  title: "кнопка должна быть красной",
  subtitle: "text",
  content: "[Text: кнопка должна быть красной]\nкнопка должна быть красной",
  kind: "text",
  ...overrides,
});

describe("InstructionInput @-mention", () => {
  it("shows all mentionItems when @ is typed with no query", () => {
    const items = [makeMentionItem(), makeMentionItem({ id: "item-2", title: "another note" })];
    render(
      <InstructionInput
        contextId="ctx-1"
        value=""
        onChange={() => {}}
        onApply={noop}
        onCancel={() => {}}
        mentionItems={items}
      />
    );
    const textarea = screen.getByPlaceholderText(/type @ to reference/i);
    fireEvent.change(textarea, { target: { value: "@", selectionStart: 1 } });
    expect(screen.getByText("кнопка должна быть красной")).toBeTruthy();
    expect(screen.getByText("another note")).toBeTruthy();
  });

  it("filters mentionItems by substring query", () => {
    const items = [
      makeMentionItem({ id: "1", title: "кнопка должна быть красной" }),
      makeMentionItem({ id: "2", title: "совсем другой текст" }),
    ];
    render(
      <InstructionInput
        contextId="ctx-1"
        value=""
        onChange={() => {}}
        onApply={noop}
        onCancel={() => {}}
        mentionItems={items}
      />
    );
    const textarea = screen.getByPlaceholderText(/type @ to reference/i);
    fireEvent.change(textarea, { target: { value: "@красной", selectionStart: 8 } });
    expect(screen.getByText("кнопка должна быть красной")).toBeTruthy();
    expect(screen.queryByText("совсем другой текст")).toBeNull();
  });

  it("selecting item via Enter attaches chip and clears @ token", () => {
    const items = [makeMentionItem()];
    const onChange = vi.fn();
    render(
      <InstructionInput
        contextId="ctx-1"
        value="@красной"
        onChange={onChange}
        onApply={noop}
        onCancel={() => {}}
        mentionItems={items}
      />
    );
    const textarea = screen.getByPlaceholderText(/type @ to reference/i);
    // Trigger the dropdown by simulating a change event
    fireEvent.change(textarea, { target: { value: "@красной", selectionStart: 8 } });
    // Press Enter to select the first item
    fireEvent.keyDown(textarea, { key: "Enter" });
    // onChange should have been called to remove the @token
    expect(onChange).toHaveBeenCalled();
    // Chip should appear
    expect(screen.getByText(/кнопка должна быть красной/)).toBeTruthy();
  });

  it("includes attached item content in onApply prompt", async () => {
    const items = [makeMentionItem()];
    const onApply = vi.fn().mockResolvedValue(undefined);
    render(
      <InstructionInput
        contextId="ctx-1"
        value="make it so"
        onChange={() => {}}
        onApply={onApply}
        onCancel={() => {}}
        mentionItems={items}
      />
    );
    const textarea = screen.getByPlaceholderText(/type @ to reference/i);
    // attach item
    fireEvent.change(textarea, { target: { value: "@красной", selectionStart: 8 } });
    fireEvent.keyDown(textarea, { key: "Enter" });
    // click Apply
    fireEvent.click(screen.getByText("Apply"));
    await vi.waitFor(() => expect(onApply).toHaveBeenCalled());
    const promptArg: string = onApply.mock.calls[0][0];
    expect(promptArg).toContain("[Text: кнопка должна быть красной]");
    expect(promptArg).toContain("make it so");
  });
});
