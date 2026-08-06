import { describe, it, expect, vi } from "vitest";
import { render, act, fireEvent } from "@testing-library/react";
import { HotkeyProvider, useHotkey, useHotkeyScope, type Scope } from "./HotkeyProvider";

function Probe({ id, fn, scope }: { id: string; fn: () => void; scope?: Scope }) {
  useHotkey(id, fn, scope ? { scope } : undefined);
  return null;
}

function ScopeActivator({ scope }: { scope: Scope }) {
  useHotkeyScope(scope, true);
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
    const here = vi.fn();
    // Register the whole terminal object group (as the terminal panel does),
    // so ⌘T opens a leader whose defaultAction (terminal.new) fires on
    // timeout rather than firing immediately as a lone bare command.
    render(
      <HotkeyProvider platform="darwin">
        <Probe id="terminal.new" fn={newTerm} />
        <Probe id="terminal.here" fn={here} />
      </HotkeyProvider>
    );
    act(() => {
      fireEvent.keyDown(window, { key: "t", metaKey: true });
    });
    expect(newTerm).toHaveBeenCalledTimes(0); // leader is pending, not fired yet
    act(() => {
      vi.advanceTimersByTime(600);
    });
    expect(newTerm).toHaveBeenCalledTimes(1);
    expect(here).toHaveBeenCalledTimes(0);
    vi.useRealTimers();
  });

  it("⌘F fires search.global immediately when files scope is NOT active", () => {
    const search = vi.fn();
    const focusSearch = vi.fn();
    render(
      <HotkeyProvider platform="darwin">
        <Probe id="search.global" fn={search} />
        <Probe id="files.focusSearch" fn={focusSearch} scope="files" />
      </HotkeyProvider>
    );
    // files scope not active -> files.focusSearch handler can't fire, so
    // mod+f is not a leader here and search.global fires on the single press.
    act(() => {
      fireEvent.keyDown(window, { key: "f", metaKey: true });
    });
    expect(search).toHaveBeenCalledTimes(1);
    expect(focusSearch).toHaveBeenCalledTimes(0);
  });

  it("⌘F with files scope active opens a leader that falls through to search.global on timeout", () => {
    vi.useFakeTimers();
    const search = vi.fn();
    const focusSearch = vi.fn();
    render(
      <HotkeyProvider platform="darwin">
        <ScopeActivator scope="files" />
        <Probe id="search.global" fn={search} />
        <Probe id="files.focusSearch" fn={focusSearch} scope="files" />
      </HotkeyProvider>
    );
    // ⌘F opens a leader for files.focusSearch (seq "f"), with search.global
    // retained as the fall-through default even though it's a different object.
    act(() => {
      fireEvent.keyDown(window, { key: "f", metaKey: true });
    });
    expect(search).toHaveBeenCalledTimes(0);
    expect(focusSearch).toHaveBeenCalledTimes(0);

    // No follow-up "f" -> timeout falls through to search.global.
    act(() => {
      vi.advanceTimersByTime(600);
    });
    expect(search).toHaveBeenCalledTimes(1);
    expect(focusSearch).toHaveBeenCalledTimes(0);
    vi.useRealTimers();
  });

  it("⌘F then f (files active) selects files.focusSearch, not the search fallthrough", () => {
    const search = vi.fn();
    const focusSearch = vi.fn();
    render(
      <HotkeyProvider platform="darwin">
        <ScopeActivator scope="files" />
        <Probe id="search.global" fn={search} />
        <Probe id="files.focusSearch" fn={focusSearch} scope="files" />
      </HotkeyProvider>
    );
    act(() => {
      fireEvent.keyDown(window, { key: "f", metaKey: true });
    });
    act(() => {
      fireEvent.keyDown(window, { key: "f" });
    });
    expect(focusSearch).toHaveBeenCalledTimes(1);
    expect(search).toHaveBeenCalledTimes(0);
  });

  it("does not open a spurious leader when nothing on the lead chord is registered", () => {
    // Nothing registered for mod+t's terminal group -> the press must not
    // open a leader nor swallow the native key.
    render(<HotkeyProvider platform="darwin" />);
    const event = new KeyboardEvent("keydown", { key: "t", metaKey: true, cancelable: true, bubbles: true });
    act(() => {
      window.dispatchEvent(event);
    });
    expect(event.defaultPrevented).toBe(false);
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
