import { describe, it, expect, vi } from "vitest";

vi.mock("@monaco-editor/react", () => ({
  loader: { config: vi.fn() },
}));

import { setupMonaco } from "./monacoSetup";
import { loader } from "@monaco-editor/react";

describe("setupMonaco", () => {
  it("configures the monaco loader once and installs MonacoEnvironment", () => {
    setupMonaco();
    setupMonaco(); // idempotent
    expect((loader.config as unknown as { mock: { calls: unknown[] } }).mock.calls.length).toBe(1);
    expect(typeof (self as unknown as { MonacoEnvironment?: { getWorker: unknown } }).MonacoEnvironment?.getWorker).toBe("function");
  });
});
