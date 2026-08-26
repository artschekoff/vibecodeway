import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("../lib/monacoSetup", () => ({ setupMonaco: vi.fn() }));

const { initVimMode, dispose } = vi.hoisted(() => {
  const dispose = vi.fn();
  return { initVimMode: vi.fn((_editor: unknown, _status: HTMLElement) => ({ dispose })), dispose };
});
vi.mock("monaco-vim", () => ({ initVimMode }));

let mountCb: ((editor: unknown) => void) | undefined;
let changeCb: ((next: string | undefined) => void) | undefined;
vi.mock("@monaco-editor/react", () => ({
  default: (props: { onMount?: (e: unknown) => void; onChange?: (v: string | undefined) => void; value: string }) => {
    mountCb = props.onMount;
    changeCb = props.onChange;
    return <div data-testid="monaco" data-value={props.value} />;
  },
}));

import { ProseEditor } from "./ProseEditor";
import { setVimMode } from "../lib/vimMode";

beforeEach(() => {
  localStorage.clear();
  vi.clearAllMocks();
  mountCb = undefined;
  changeCb = undefined;
});

describe("ProseEditor with vim mode off", () => {
  it("renders a plain textarea and forwards every prop", () => {
    const onChange = vi.fn();
    render(<ProseEditor value="hello" onChange={onChange} aria-label="notes" rows={4} placeholder="type here" />);

    const box = screen.getByLabelText("notes") as HTMLTextAreaElement;
    expect(box.tagName).toBe("TEXTAREA");
    expect(box.value).toBe("hello");
    expect(box.rows).toBe(4);
    expect(box.placeholder).toBe("type here");
    expect(screen.queryByTestId("monaco")).not.toBeInTheDocument();
  });

  it("fires the call site's onChange with the textarea event", () => {
    // Read the value inside the handler: this is a controlled textarea, so React
    // resets the DOM node's value before the assertion below would see it.
    const seen: string[] = [];
    render(<ProseEditor value="" onChange={(e) => seen.push(e.target.value)} aria-label="notes" />);

    fireEvent.change(screen.getByLabelText("notes"), { target: { value: "typed" } });
    expect(seen).toEqual(["typed"]);
  });

  it("does not load monaco-vim at all", () => {
    render(<ProseEditor value="" onChange={vi.fn()} aria-label="notes" />);
    expect(initVimMode).not.toHaveBeenCalled();
  });
});

describe("ProseEditor with vim mode on", () => {
  beforeEach(() => { setVimMode(true); });

  it("renders monaco instead of a textarea and binds vim to it", async () => {
    render(<ProseEditor value="hello" onChange={vi.fn()} aria-label="notes" />);

    expect(screen.getByTestId("monaco")).toBeInTheDocument();
    expect(document.querySelector("textarea")).toBeNull();

    const editor = { id: "ed" };
    act(() => { mountCb?.(editor); });
    await waitFor(() => expect(initVimMode).toHaveBeenCalled());
    expect(initVimMode.mock.calls[0][0]).toBe(editor);
    expect(initVimMode.mock.calls[0][1]).toBe(screen.getByTestId("vim-status"));
  });

  it("adapts monaco's change into the textarea-shaped event call sites expect", () => {
    const onChange = vi.fn();
    render(<ProseEditor value="" onChange={onChange} aria-label="notes" />);

    act(() => { changeCb?.("from monaco"); });
    expect(onChange.mock.calls[0][0].target.value).toBe("from monaco");
  });

  it("treats an undefined monaco value as empty text", () => {
    const onChange = vi.fn();
    render(<ProseEditor value="x" onChange={onChange} aria-label="notes" />);

    act(() => { changeCb?.(undefined); });
    expect(onChange.mock.calls[0][0].target.value).toBe("");
  });

  it("disposes the vim binding on unmount", async () => {
    const { unmount } = render(<ProseEditor value="" onChange={vi.fn()} aria-label="notes" />);
    act(() => { mountCb?.({ id: "ed" }); });
    await waitFor(() => expect(initVimMode).toHaveBeenCalled());

    unmount();
    expect(dispose).toHaveBeenCalled();
  });

  it("swaps back to a textarea when vim is switched off", async () => {
    render(<ProseEditor value="hello" onChange={vi.fn()} aria-label="notes" />);
    act(() => { mountCb?.({ id: "ed" }); });
    await waitFor(() => expect(initVimMode).toHaveBeenCalled());

    act(() => { setVimMode(false); });

    await waitFor(() => expect((screen.getByLabelText("notes") as HTMLElement).tagName).toBe("TEXTAREA"));
    expect(dispose).toHaveBeenCalled();
  });
});
