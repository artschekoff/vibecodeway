import { describe, it, expect, vi } from "vitest";
import { render, act, fireEvent } from "@testing-library/react";
import { HotkeyProvider, useHotkey } from "./HotkeyProvider";

function Probe({ id, fn }: { id: string; fn: () => void }) {
  useHotkey(id, fn);
  return null;
}

describe("HotkeyProvider", () => {
  it("fires a single-chord command", () => {
    const fn = vi.fn();
    render(
      <HotkeyProvider platform="darwin">
        <Probe id="search.global" fn={fn} />
      </HotkeyProvider>
    );
    act(() => {
      fireEvent.keyDown(window, { key: "f", metaKey: true });
    });
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it("fires the sequence child on leader + letter", () => {
    const here = vi.fn();
    render(
      <HotkeyProvider platform="darwin">
        <Probe id="terminal.here" fn={here} />
      </HotkeyProvider>
    );
    act(() => {
      fireEvent.keyDown(window, { key: "t", metaKey: true });
    });
    act(() => {
      fireEvent.keyDown(window, { key: "h" });
    });
    expect(here).toHaveBeenCalledTimes(1);
  });

  it("fires the default action on leader timeout", () => {
    vi.useFakeTimers();
    const newTerm = vi.fn();
    render(
      <HotkeyProvider platform="darwin">
        <Probe id="terminal.new" fn={newTerm} />
      </HotkeyProvider>
    );
    act(() => {
      fireEvent.keyDown(window, { key: "t", metaKey: true });
    });
    act(() => {
      vi.advanceTimersByTime(600);
    });
    expect(newTerm).toHaveBeenCalledTimes(1);
    vi.useRealTimers();
  });

  it("does not preventDefault a lead chord whose only bindings are sequenced (no bare/default handler)", () => {
    const insert = vi.fn();
    render(
      <HotkeyProvider platform="darwin">
        <Probe id="context.copyName" fn={insert} />
      </HotkeyProvider>
    );
    // context.copyName is mod+c sequence "n" with no defaultAction sibling
    // on that lead in the registry, so the native ⌘C keypress must not be
    // swallowed.
    const event = new KeyboardEvent("keydown", { key: "c", metaKey: true, cancelable: true, bubbles: true });
    act(() => {
      window.dispatchEvent(event);
    });
    expect(event.defaultPrevented).toBe(false);
  });

  it("ignores plain-key chords typed inside an input, but allows mod chords", () => {
    const search = vi.fn();
    render(
      <HotkeyProvider platform="darwin">
        <Probe id="search.global" fn={search} />
        <input data-testid="txt" />
      </HotkeyProvider>
    );
    const input = document.querySelector("input")!;

    // A plain key with no mod/ctrl must never reach the leader/dispatch
    // logic while typing in an input (it's real text entry).
    act(() => {
      fireEvent.keyDown(input, { key: "f" });
    });
    expect(search).toHaveBeenCalledTimes(0);

    // The same key WITH mod is allowed even while typing.
    act(() => {
      fireEvent.keyDown(input, { key: "f", metaKey: true });
    });
    expect(search).toHaveBeenCalledTimes(1);
  });

  it("emits a hotkey:<id> CustomEvent when a command fires", () => {
    const fn = vi.fn();
    const listener = vi.fn();
    window.addEventListener("hotkey:search.global", listener);
    render(
      <HotkeyProvider platform="darwin">
        <Probe id="search.global" fn={fn} />
      </HotkeyProvider>
    );
    act(() => {
      fireEvent.keyDown(window, { key: "f", metaKey: true });
    });
    expect(listener).toHaveBeenCalledTimes(1);
    window.removeEventListener("hotkey:search.global", listener);
  });
});
