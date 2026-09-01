import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent, waitFor, screen } from "@testing-library/react";

const H = vi.hoisted(() => ({
  GetKeybindings: vi.fn(),
  ListAllContexts: vi.fn(),
  ListOrganizations: vi.fn(),
}));
vi.mock("../wailsjs/go/main/App", () => H);

const R = vi.hoisted(() => ({
  Environment: vi.fn(),
  EventsOn: vi.fn(() => vi.fn()),
  EventsOff: vi.fn(),
  SendNotification: vi.fn(),
  InitializeNotifications: vi.fn(),
}));
vi.mock("../wailsjs/runtime/runtime", () => R);

import App from "./App";
import { SIDEBAR_RAIL_KEY, RAIL_MIN, RAIL_MAX } from "./App";

// Node 25 ships a native `localStorage` global without `.clear()`, shadowing
// jsdom's — same replacement the ContextList specs use.
function makeLocalStorageMock(): Storage {
  let store: Record<string, string> = {};
  return {
    get length() { return Object.keys(store).length; },
    key(i: number) { return Object.keys(store)[i] ?? null; },
    getItem(k: string) { return Object.prototype.hasOwnProperty.call(store, k) ? store[k] : null; },
    setItem(k: string, v: string) { store[k] = String(v); },
    removeItem(k: string) { delete store[k]; },
    clear() { store = {}; },
  } as Storage;
}
const localStorageMock = makeLocalStorageMock();

async function renderApp() {
  const { container } = render(<App />);
  await waitFor(() => expect(H.GetKeybindings).toHaveBeenCalled());
  return container;
}
const rail = (c: HTMLElement) => c.querySelector(".app-sidebar") as HTMLElement;

// Drag the rail's separator by `dx` px.
function drag(dx: number) {
  const handle = screen.getByRole("separator", { name: "resize sidebar rail" });
  fireEvent.mouseDown(handle, { clientX: 100 });
  fireEvent.mouseMove(window, { clientX: 100 + dx });
  fireEvent.mouseUp(window);
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal("localStorage", localStorageMock);
  localStorageMock.clear();
  H.GetKeybindings.mockResolvedValue([]);
  H.ListAllContexts.mockResolvedValue([]);
  H.ListOrganizations.mockResolvedValue([]);
  R.Environment.mockResolvedValue({ platform: "darwin", buildType: "dev", arch: "arm64" });
});

describe("sidebar rail width", () => {
  it("widens on drag so full context names have room", async () => {
    const c = await renderApp();
    const before = parseInt(rail(c).style.width, 10);
    drag(120);
    expect(parseInt(rail(c).style.width, 10)).toBe(before + 120);
  });

  it("clamps to the icon-rail minimum and the readable maximum", async () => {
    const c = await renderApp();
    drag(-9999);
    expect(parseInt(rail(c).style.width, 10)).toBe(RAIL_MIN);
    drag(9999);
    expect(parseInt(rail(c).style.width, 10)).toBe(RAIL_MAX);
  });

  // The rail width alone is not enough: .tab-item__title caps at a fixed width,
  // so without this the names stay truncated however wide the rail gets.
  it("publishes the width as --rail-w, which the tab titles size against", async () => {
    const c = await renderApp();
    expect(rail(c).style.getPropertyValue("--rail-w")).toBe(`${RAIL_MIN}px`);
    drag(150);
    expect(rail(c).style.getPropertyValue("--rail-w")).toBe(`${RAIL_MIN + 150}px`);
  });

  it("collapses to the icon rail and back on click", async () => {
    const c = await renderApp();
    drag(150);
    const expanded = rail(c).style.width;

    fireEvent.click(screen.getByRole("button", { name: "Collapse sidebar" }));
    expect(rail(c).style.width).toBe(`${RAIL_MIN}px`);

    // Restores the width the user had dragged to, not a fixed default.
    fireEvent.click(screen.getByRole("button", { name: "Expand sidebar" }));
    expect(rail(c).style.width).toBe(expanded);
  });

  it("expands to a readable default when it has never been widened", async () => {
    const c = await renderApp();
    expect(rail(c).style.width).toBe(`${RAIL_MIN}px`);
    fireEvent.click(screen.getByRole("button", { name: "Expand sidebar" }));
    expect(parseInt(rail(c).style.width, 10)).toBeGreaterThan(RAIL_MIN);
  });

  it("persists the width across mounts", async () => {
    const c = await renderApp();
    drag(100);
    const widened = rail(c).style.width;
    expect(localStorage.getItem(SIDEBAR_RAIL_KEY)).toBe(parseInt(widened, 10).toString());

    const c2 = await renderApp();
    expect(rail(c2).style.width).toBe(widened);
  });
});
