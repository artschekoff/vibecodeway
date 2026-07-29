import { createContext, useCallback, useContext, useEffect, useState } from "react";
import { GetEditorSettings, SetEditorSettings } from "../../wailsjs/go/main/App";

export type EditorTheme = "light" | "dark" | "high-contrast";

export type EditorSettings = {
  theme: EditorTheme;
  wordWrap: boolean;
  fontSize: number;
  minimap: boolean;
  lineNumbers: "on" | "off" | "relative";
  tabSize: number;
  renderWhitespace: "none" | "boundary" | "all";
  diffSideBySide: boolean;
};

// Defaults preserve the editor's current look (light theme, size 12, side-by-side).
export const DEFAULT_EDITOR_SETTINGS: EditorSettings = {
  theme: "light",
  wordWrap: false,
  fontSize: 12,
  minimap: false,
  lineNumbers: "on",
  tabSize: 2,
  renderWhitespace: "none",
  diffSideBySide: true,
};

const MONACO_THEME: Record<EditorTheme, string> = {
  light: "vs",
  dark: "vs-dark",
  "high-contrast": "hc-black",
};

/** Maps our settings to the Monaco DiffEditor `theme` + `options` props. */
export function toMonacoOptions(s: EditorSettings): { theme: string; options: Record<string, unknown> } {
  return {
    theme: MONACO_THEME[s.theme],
    options: {
      readOnly: false,
      originalEditable: false,
      renderSideBySide: s.diffSideBySide,
      wordWrap: s.wordWrap ? "on" : "off",
      fontSize: s.fontSize,
      minimap: { enabled: s.minimap },
      lineNumbers: s.lineNumbers,
      tabSize: s.tabSize,
      renderWhitespace: s.renderWhitespace,
    },
  };
}

type Ctx = { settings: EditorSettings; update: (patch: Partial<EditorSettings>) => void };
const EditorSettingsContext = createContext<Ctx | null>(null);

export function EditorSettingsProvider({ children }: { children: React.ReactNode }) {
  const [settings, setSettings] = useState<EditorSettings>(DEFAULT_EDITOR_SETTINGS);

  // Load the persisted blob once; merge over defaults so a missing/corrupt/old
  // value never breaks the editor.
  useEffect(() => {
    let active = true;
    void GetEditorSettings()
      .then((raw) => {
        if (!active || !raw) return;
        try {
          setSettings({ ...DEFAULT_EDITOR_SETTINGS, ...(JSON.parse(raw) as Partial<EditorSettings>) });
        } catch {
          /* keep defaults */
        }
      })
      .catch(() => {});
    return () => { active = false; };
  }, []);

  const update = useCallback((patch: Partial<EditorSettings>) => {
    setSettings((prev) => {
      const next = { ...prev, ...patch };
      void SetEditorSettings(JSON.stringify(next)).catch(() => {});
      return next;
    });
  }, []);

  return <EditorSettingsContext.Provider value={{ settings, update }}>{children}</EditorSettingsContext.Provider>;
}

export function useEditorSettings(): Ctx {
  const ctx = useContext(EditorSettingsContext);
  if (!ctx) throw new Error("useEditorSettings must be used within EditorSettingsProvider");
  return ctx;
}
