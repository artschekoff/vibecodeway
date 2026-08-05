import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";

const {
  ListContextsByOrg,
  AuthStatus,
  GetPendingCodeReviews,
  ListTrackedRepos,
  GetSupportedPlatforms,
  ListOrganizations,
  ListWorkflowStatuses,
  SetContextWorkflowStatus,
} = vi.hoisted(() => ({
  ListContextsByOrg: vi.fn(),
  AuthStatus: vi.fn(),
  GetPendingCodeReviews: vi.fn(),
  ListTrackedRepos: vi.fn(),
  GetSupportedPlatforms: vi.fn(),
  ListOrganizations: vi.fn(),
  ListWorkflowStatuses: vi.fn(),
  SetContextWorkflowStatus: vi.fn(),
}));

vi.mock("../../wailsjs/go/main/App", () => ({
  ListContextsByOrg,
  AuthStatus: AuthStatus.mockResolvedValue("ok"),
  GetPendingCodeReviews: GetPendingCodeReviews.mockResolvedValue([]),
  ListTrackedRepos: ListTrackedRepos.mockResolvedValue([]),
  GetSupportedPlatforms: GetSupportedPlatforms.mockResolvedValue([]),
  ListOrganizations: ListOrganizations.mockResolvedValue([]),
  ListWorkflowStatuses: ListWorkflowStatuses.mockResolvedValue([]),
  HasSlackPassphrase: vi.fn().mockResolvedValue(true),
  ValidateSlackPassphrase: vi.fn(),
  SetContextOrg: vi.fn(),
  SetContextsOrder: vi.fn(),
  SetContextWorkflowStatus,
}));

vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: () => () => {},
  EventsOff: () => {},
  BrowserOpenURL: () => {},
}));

import { TabProvider } from "../contexts/TabContext";
import { PassphraseProvider } from "../contexts/PassphraseContext";
import ContextList from "./ContextList";

// Node 25 exposes its own native `localStorage` global that lacks `.clear()`,
// shadowing jsdom's implementation. Replace it with a proper in-memory mock
// (same pattern as TabContext.test.tsx).
function makeLocalStorageMock(): Storage {
  let store: Record<string, string> = {};
  return {
    get length() { return Object.keys(store).length; },
    key(index: number) { return Object.keys(store)[index] ?? null; },
    getItem(k: string) { return Object.prototype.hasOwnProperty.call(store, k) ? store[k] : null; },
    setItem(k: string, v: string) { store[k] = String(v); },
    removeItem(k: string) { delete store[k]; },
    clear() { store = {}; },
  } as Storage;
}

const localStorageMock = makeLocalStorageMock();

function renderList() {
  return render(
    <TabProvider>
      <PassphraseProvider>
        <ContextList orgId="" tabId="tab-1" />
      </PassphraseProvider>
    </TabProvider>
  );
}

beforeEach(() => {
  ListContextsByOrg.mockReset();
  vi.stubGlobal("localStorage", localStorageMock);
  localStorageMock.clear();
});

describe("ContextList importance column", () => {
  it("renders importance value and a dash fallback when missing", async () => {
    ListContextsByOrg.mockResolvedValue([
      {
        id: "c1",
        key: "PROJ-1",
        status: "active",
        orgId: "",
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
        snapshotStatus: "not_started",
        snapshotIsStale: false,
        importance: 3.5,
      },
      {
        id: "c2",
        key: "PROJ-2",
        status: "active",
        orgId: "",
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
        snapshotStatus: "not_started",
        snapshotIsStale: false,
      },
    ]);

    renderList();

    expect(await screen.findByText("3.5")).toBeInTheDocument();

    const row2 = screen.getByText("PROJ-2").closest("tr");
    expect(row2).not.toBeNull();
    const headers = Array.from(document.querySelectorAll("thead th")).map((th) => th.textContent);
    const importanceIdx = headers.indexOf("Importance");
    expect(importanceIdx).toBeGreaterThan(-1);
    const cells = row2 ? Array.from(row2.querySelectorAll("td")) : [];
    expect(cells[importanceIdx].textContent).toBe("—");
  });
});

describe("ContextList status change", () => {
  it("changes a context's workflow status when a new option is picked", async () => {
    ListWorkflowStatuses.mockResolvedValue([
      { slug: "not_started", label: "Not started" },
      { slug: "in_progress", label: "In Progress" },
    ]);
    ListContextsByOrg.mockResolvedValue([
      {
        id: "c1", key: "PROJ-1", status: "active", orgId: "",
        createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z",
        snapshotStatus: "not_started", snapshotIsStale: false,
      },
    ]);

    renderList();

    const select = (await screen.findByLabelText("Change status")) as HTMLSelectElement;
    expect(select.value).toBe("not_started");
    fireEvent.change(select, { target: { value: "in_progress" } });

    expect(SetContextWorkflowStatus).toHaveBeenCalledWith("c1", "in_progress", "");
  });
});
