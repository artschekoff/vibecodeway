import { describe, it, expect, vi } from "vitest";
import { render, act, fireEvent } from "@testing-library/react";
import { HotkeyProvider, useHotkey, useHotkeyScope, useModHeld, type Scope } from "./HotkeyProvider";
import { TabVisibleContext, useTabVisible } from "../contexts/TabVisibleContext";

function Probe({ id, fn, scope }: { id: string; fn: () => void; scope?: Scope }) {
  useHotkey(id, fn, scope ? { scope } : undefined);
  return null;
}

// Mirrors how per-tab components (TerminalPanel etc.) register their handlers.
function TabProbe({ id, fn }: { id: string; fn: () => void }) {
  useHotkey(id, fn, { scope: "global", enabled: useTabVisible() });
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
        <Probe id="context.filterItems" fn={fn} />
      </HotkeyProvider>
    );
    act(() => {
      fireEvent.keyDown(window, { key: "f", metaKey: true });
    });
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it("fires the sequence child on leader + letter", () => {
    const focusDown = vi.fn();
    render(
      <HotkeyProvider platform="darwin">
        <ScopeActivator scope="editor" />
        <Probe id="editor.focusDown" fn={focusDown} scope="editor" />
      </HotkeyProvider>
    );
    act(() => {
      fireEvent.keyDown(window, { key: "e", metaKey: true });
    });
    act(() => {
      fireEvent.keyDown(window, { key: "j" });
    });
    expect(focusDown).toHaveBeenCalledTimes(1);
  });

  it("fires the default action on leader timeout", () => {
    vi.useFakeTimers();
    const openEditor = vi.fn();
    const focusDown = vi.fn();
    // Register the whole editor object group, so ⌘E opens a leader whose bare
    // fall-through (editor.open) fires on timeout rather than firing immediately.
    render(
      <HotkeyProvider platform="darwin">
        <ScopeActivator scope="editor" />
        <Probe id="editor.open" fn={openEditor} />
        <Probe id="editor.focusDown" fn={focusDown} scope="editor" />
      </HotkeyProvider>
    );
    act(() => {
      fireEvent.keyDown(window, { key: "e", metaKey: true });
    });
    expect(openEditor).toHaveBeenCalledTimes(0); // leader is pending, not fired yet
    act(() => {
      vi.advanceTimersByTime(600);
    });
    expect(openEditor).toHaveBeenCalledTimes(1);
    expect(focusDown).toHaveBeenCalledTimes(0);
    vi.useRealTimers();
  });

  it("fires ⌘T immediately — it is a bare chord, not a leader", () => {
    vi.useFakeTimers();
    const newTerm = vi.fn();
    render(
      <HotkeyProvider platform="darwin">
        <Probe id="terminal.new" fn={newTerm} />
        <Probe id="terminal.here" fn={vi.fn()} />
        <Probe id="terminal.insert" fn={vi.fn()} />
      </HotkeyProvider>
    );
    act(() => {
      fireEvent.keyDown(window, { key: "t", metaKey: true });
    });
    expect(newTerm).toHaveBeenCalledTimes(1); // no timer advance needed
    vi.useRealTimers();
  });

  it("⌘E + k moves editor zone focus when editor scope is active", () => {
    const open = vi.fn();
    const focusUp = vi.fn();
    render(
      <HotkeyProvider platform="darwin">
        <ScopeActivator scope="editor" />
        <Probe id="editor.open" fn={open} />
        <Probe id="editor.focusUp" fn={focusUp} scope="editor" />
      </HotkeyProvider>
    );
    act(() => {
      fireEvent.keyDown(window, { key: "e", metaKey: true });
    });
    act(() => {
      fireEvent.keyDown(window, { key: "k" });
    });
    expect(focusUp).toHaveBeenCalledTimes(1);
    expect(open).toHaveBeenCalledTimes(0);
  });

  it("⌘E opens the editor immediately when editor scope is NOT active", () => {
    const open = vi.fn();
    const focusUp = vi.fn();
    render(
      <HotkeyProvider platform="darwin">
        <Probe id="editor.open" fn={open} />
        <Probe id="editor.focusUp" fn={focusUp} scope="editor" />
      </HotkeyProvider>
    );
    act(() => {
      fireEvent.keyDown(window, { key: "e", metaKey: true });
    });
    expect(open).toHaveBeenCalledTimes(1);
    expect(focusUp).toHaveBeenCalledTimes(0);
  });

  it("⌘F fires context.filterItems immediately when files scope is NOT active", () => {
    const search = vi.fn();
    const focusSearch = vi.fn();
    render(
      <HotkeyProvider platform="darwin">
        <Probe id="context.filterItems" fn={search} />
        <Probe id="files.focusSearch" fn={focusSearch} scope="files" />
      </HotkeyProvider>
    );
    // files scope not active -> files.focusSearch handler can't fire, so
    // mod+f is not a leader here and context.filterItems fires on the single press.
    act(() => {
      fireEvent.keyDown(window, { key: "f", metaKey: true });
    });
    expect(search).toHaveBeenCalledTimes(1);
    expect(focusSearch).toHaveBeenCalledTimes(0);
  });

  it("⌘F with files scope active opens a leader that falls through to context.filterItems on timeout", () => {
    vi.useFakeTimers();
    const search = vi.fn();
    const focusSearch = vi.fn();
    render(
      <HotkeyProvider platform="darwin">
        <ScopeActivator scope="files" />
        <Probe id="context.filterItems" fn={search} />
        <Probe id="files.focusSearch" fn={focusSearch} scope="files" />
      </HotkeyProvider>
    );
    // ⌘F opens a leader for files.focusSearch (seq "f"), with context.filterItems
    // retained as the fall-through default even though it's a different object.
    act(() => {
      fireEvent.keyDown(window, { key: "f", metaKey: true });
    });
    expect(search).toHaveBeenCalledTimes(0);
    expect(focusSearch).toHaveBeenCalledTimes(0);

    // No follow-up "f" -> timeout falls through to context.filterItems.
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
        <Probe id="context.filterItems" fn={search} />
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
        <Probe id="context.filterItems" fn={search} />
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
    window.addEventListener("hotkey:context.filterItems", listener);
    render(
      <HotkeyProvider platform="darwin">
        <Probe id="context.filterItems" fn={fn} />
      </HotkeyProvider>
    );
    act(() => {
      fireEvent.keyDown(window, { key: "f", metaKey: true });
    });
    expect(listener).toHaveBeenCalledTimes(1);
    window.removeEventListener("hotkey:context.filterItems", listener);
  });

  // Regression: editor tabs stay mounted (display:none) when inactive, so a
  // hidden tab's TerminalPanel also registers global terminal.* handlers.
  // Handlers gated on TabVisibleContext must win over hidden ones regardless
  // of registration order (⌘I opened the insert menu in a hidden tab).
  it("a hidden tab's handler does not steal a global command from the visible tab", () => {
    const hidden = vi.fn();
    const visible = vi.fn();
    render(
      <HotkeyProvider platform="darwin">
        <TabVisibleContext.Provider value={false}>
          <TabProbe id="terminal.insert" fn={hidden} />
        </TabVisibleContext.Provider>
        <TabVisibleContext.Provider value={true}>
          <TabProbe id="terminal.insert" fn={visible} />
        </TabVisibleContext.Provider>
      </HotkeyProvider>
    );
    act(() => {
      fireEvent.keyDown(window, { key: "i", metaKey: true });
    });
    expect(hidden).not.toHaveBeenCalled();
    expect(visible).toHaveBeenCalledTimes(1);
  });

  it("useModHeld tracks the ⌘ key and ⌘R fires reviews.refresh", () => {
    const fn = vi.fn();
    function ModProbe() { return <span data-testid="mod">{useModHeld() ? "held" : "idle"}</span>; }
    const { getByTestId } = render(
      <HotkeyProvider platform="darwin">
        <Probe id="reviews.refresh" fn={fn} />
        <ModProbe />
      </HotkeyProvider>
    );
    expect(getByTestId("mod").textContent).toBe("idle");
    act(() => { fireEvent.keyDown(window, { key: "Meta", metaKey: true }); });
    expect(getByTestId("mod").textContent).toBe("held");
    act(() => { fireEvent.keyDown(window, { key: "r", metaKey: true }); });
    expect(fn).toHaveBeenCalledTimes(1);
    act(() => { fireEvent.keyUp(window, { key: "Meta", metaKey: false }); });
    expect(getByTestId("mod").textContent).toBe("idle");
  });

  it("useHotkey is a no-op outside a provider", () => {
    expect(() => render(<Probe id="reviews.refresh" fn={vi.fn()} />)).not.toThrow();
  });
});
