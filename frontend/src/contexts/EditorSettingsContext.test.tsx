import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { GetEditorSettings, SetEditorSettings } = vi.hoisted(() => ({
  GetEditorSettings: vi.fn(), SetEditorSettings: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => ({ GetEditorSettings, SetEditorSettings }));

import {
  EditorSettingsProvider, useEditorSettings, toMonacoOptions, DEFAULT_EDITOR_SETTINGS,
} from "./EditorSettingsContext";

function Probe() {
  const { settings, update } = useEditorSettings();
  return (
    <div>
      <span data-testid="theme">{settings.theme}</span>
      <span data-testid="wrap">{String(settings.wordWrap)}</span>
      <button onClick={() => update({ theme: "dark" })}>go-dark</button>
    </div>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  SetEditorSettings.mockResolvedValue(undefined);
});

describe("EditorSettingsContext", () => {
  it("merges a partial stored blob over defaults", async () => {
    GetEditorSettings.mockResolvedValue(JSON.stringify({ wordWrap: true }));
    render(<EditorSettingsProvider><Probe /></EditorSettingsProvider>);
    await waitFor(() => expect(screen.getByTestId("wrap")).toHaveTextContent("true"));
    // Unspecified fields fall back to defaults.
    expect(screen.getByTestId("theme")).toHaveTextContent(DEFAULT_EDITOR_SETTINGS.theme);
  });

  it("keeps defaults when the stored blob is corrupt", async () => {
    GetEditorSettings.mockResolvedValue("{not json");
    render(<EditorSettingsProvider><Probe /></EditorSettingsProvider>);
    await waitFor(() => expect(screen.getByTestId("theme")).toHaveTextContent(DEFAULT_EDITOR_SETTINGS.theme));
  });

  it("update applies and persists", async () => {
    GetEditorSettings.mockResolvedValue("");
    render(<EditorSettingsProvider><Probe /></EditorSettingsProvider>);
    fireEvent.click(screen.getByText("go-dark"));
    await waitFor(() => expect(screen.getByTestId("theme")).toHaveTextContent("dark"));
    expect(SetEditorSettings).toHaveBeenCalledWith(expect.stringContaining('"theme":"dark"'));
  });

  it("toMonacoOptions maps theme, wrap and diff layout", () => {
    const { theme, options } = toMonacoOptions({ ...DEFAULT_EDITOR_SETTINGS, theme: "dark", wordWrap: true, diffSideBySide: false });
    expect(theme).toBe("vs-dark");
    expect(options.wordWrap).toBe("on");
    expect(options.renderSideBySide).toBe(false);
  });
});
