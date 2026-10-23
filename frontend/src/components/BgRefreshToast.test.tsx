import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { BgRefreshToast } from "./BgRefreshToast";

const handlers = new Map<string, (p?: unknown) => void>();
vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: (name: string, cb: (p?: unknown) => void) => { handlers.set(name, cb); return () => handlers.delete(name); },
  EventsOff: (name: string) => handlers.delete(name),
}));

const emit = (name: string, payload?: unknown) => act(() => { handlers.get(name)?.(payload); });

describe("BgRefreshToast", () => {
  beforeEach(() => {
    handlers.clear();
    (window as unknown as Record<string, unknown>).go = {};
  });

  it("tracks parallel contexts and drops them on done/error", () => {
    render(<BgRefreshToast />);
    emit("bg-refresh:contexts:start");
    emit("bg-refresh:contexts:progress", { current: 1, total: 42, contextKey: "ctx-a", step: "jira" });
    emit("bg-refresh:contexts:progress", { current: 1, total: 42, contextKey: "ctx-b", step: "vcs" });

    expect(screen.getByText(/Updating contexts… 1\/42/)).toBeInTheDocument();
    expect(screen.getByText(/2 in flight/)).toBeInTheDocument();

    fireEvent.click(screen.getByText(/Updating contexts/));
    expect(screen.getByText("ctx-a")).toBeInTheDocument();
    expect(screen.getByText("ctx-b")).toBeInTheDocument();
    expect(screen.getByText("jira")).toBeInTheDocument();

    emit("bg-refresh:contexts:progress", { current: 2, total: 42, contextKey: "ctx-a", step: "done" });
    expect(screen.queryByText("ctx-a")).not.toBeInTheDocument();
    expect(screen.getByText("ctx-b")).toBeInTheDocument();

    emit("bg-refresh:contexts:done", { total: 42, errors: 1 });
    expect(screen.getByText(/41\/42 contexts updated, 1 error/)).toBeInTheDocument();
  });
});
