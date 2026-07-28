import { useEffect } from "react";
import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Tab } from "../types";

// Controllable tab state.
let tabState: { tabs: Tab[]; activeTabId: string };
vi.mock("../contexts/TabContext", () => ({ useTabContext: () => tabState }));

// Track EditorWorkspace mount/unmount to prove keep-alive.
const editorMounts: string[] = [];
const editorUnmounts: string[] = [];
vi.mock("../pages/EditorWorkspace", () => ({
  EditorWorkspace: ({ contextId }: { contextId: string }) => {
    // Record mount/unmount via a ref-less effect stub.
    return <EditorStub contextId={contextId} />;
  },
}));
function EditorStub({ contextId }: { contextId: string }) {
  useEffect(() => {
    editorMounts.push(contextId);
    return () => { editorUnmounts.push(contextId); };
  }, [contextId]);
  return <div data-testid={`editor-${contextId}`}>{contextId}</div>;
}

// Stub the non-editor page components TabContent imports.
vi.mock("../pages/ContextList", () => ({ default: () => <div data-testid="org" /> }));
vi.mock("../pages/ContextDetail", () => ({ default: () => <div data-testid="context" /> }));
vi.mock("../pages/Settings", () => ({ default: () => <div data-testid="settings" /> }));
vi.mock("../pages/AppLogsPage", () => ({ default: () => <div data-testid="logs" /> }));
vi.mock("../pages/PromptsPage", () => ({ default: () => <div data-testid="prompts" /> }));
vi.mock("../pages/MyBoardPage", () => ({ default: () => <div data-testid="board" /> }));
vi.mock("../pages/AskAIPage", () => ({ default: () => <div data-testid="ask" /> }));
vi.mock("../pages/ActivityTimeline", () => ({ default: () => <div data-testid="timeline" /> }));
vi.mock("../pages/ActivityFeedPage", () => ({ default: () => <div data-testid="activity-feed" /> }));

import { TabContent } from "./TabContent";

const editorTab: Tab = { id: "e1", type: "editor", contextId: "c1", contextKey: "K-1", title: "K-1 — Editor", pinned: false } as Tab;
const settingsTab: Tab = { id: "s1", type: "settings", title: "Settings", pinned: false } as Tab;

beforeEach(() => {
  editorMounts.length = 0;
  editorUnmounts.length = 0;
});

describe("TabContent keep-alive", () => {
  it("keeps an editor tab mounted (hidden) when switching to another tab", () => {
    tabState = { tabs: [editorTab, settingsTab], activeTabId: "e1" };
    const { rerender } = render(<TabContent />);
    expect(screen.getByTestId("editor-c1")).toBeVisible();

    // Switch to the settings tab.
    tabState = { tabs: [editorTab, settingsTab], activeTabId: "s1" };
    rerender(<TabContent />);

    // Editor must NOT have unmounted — it stays in the DOM, just hidden.
    expect(editorUnmounts).not.toContain("c1");
    expect(screen.getByTestId("editor-c1")).not.toBeVisible();
    expect(screen.getByTestId("settings")).toBeInTheDocument();
  });

  it("unmounts the editor only when its tab is closed", () => {
    tabState = { tabs: [editorTab, settingsTab], activeTabId: "e1" };
    const { rerender } = render(<TabContent />);
    expect(editorMounts).toContain("c1");

    // Close the editor tab (removed from tabs), settings becomes active.
    tabState = { tabs: [settingsTab], activeTabId: "s1" };
    rerender(<TabContent />);

    expect(editorUnmounts).toContain("c1");
    expect(screen.queryByTestId("editor-c1")).not.toBeInTheDocument();
  });
});
