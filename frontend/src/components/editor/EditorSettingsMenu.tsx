import { useState } from "react";
import { Settings } from "lucide-react";
import { useEditorSettings, type EditorTheme } from "../../contexts/EditorSettingsContext";
import { readVimMode, setVimMode } from "../../lib/vimMode";

const iconBtn: React.CSSProperties = {
  display: "flex", alignItems: "center", background: "none", border: "none",
  cursor: "pointer", color: "var(--text-muted)", padding: 2,
};
const popover: React.CSSProperties = {
  position: "absolute", top: "100%", right: 0, marginTop: 4, zIndex: 50,
  minWidth: 220, background: "var(--bg-panel, var(--bg))", border: "1px solid var(--border)",
  borderRadius: 6, padding: 8, display: "flex", flexDirection: "column", gap: 8,
  boxShadow: "0 6px 20px rgba(0,0,0,0.3)", fontSize: 11, color: "var(--text)",
};
const rowStyle: React.CSSProperties = { display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8 };
const selectStyle: React.CSSProperties = { fontSize: 11 };
const segBtn = (active: boolean): React.CSSProperties => ({
  fontSize: 11, padding: "2px 6px", cursor: "pointer",
  border: "1px solid var(--border)", borderRadius: 4,
  background: active ? "var(--accent, #2563eb)" : "transparent",
  color: active ? "#fff" : "var(--text)",
});

const THEMES: EditorTheme[] = ["light", "dark", "high-contrast"];
const THEME_LABEL: Record<EditorTheme, string> = { light: "Light", dark: "Dark", "high-contrast": "High" };

export function EditorSettingsMenu() {
  const { settings, update } = useEditorSettings();
  const [open, setOpen] = useState(false);
  // Vim lives in localStorage (app-wide, shared with Settings page and ProseEditor).
  const [vimOn, setVimOn] = useState(readVimMode);

  return (
    <div style={{ position: "relative" }}>
      <button aria-label="editor settings" title="Editor settings" onClick={() => setOpen((o) => !o)} style={iconBtn}>
        <Settings size={13} />
      </button>
      {open && (
        <>
          {/* click-away */}
          <div onClick={() => setOpen(false)} style={{ position: "fixed", inset: 0, zIndex: 40 }} />
          <div role="menu" style={popover} onClick={(e) => e.stopPropagation()}>
            <div style={rowStyle}>
              <span>Theme</span>
              <div style={{ display: "flex", gap: 2 }}>
                {THEMES.map((t) => (
                  <button key={t} style={segBtn(settings.theme === t)} onClick={() => update({ theme: t })}>
                    {THEME_LABEL[t]}
                  </button>
                ))}
              </div>
            </div>

            <label style={rowStyle}>
              <span>Word wrap</span>
              <input type="checkbox" checked={settings.wordWrap} onChange={(e) => update({ wordWrap: e.target.checked })} />
            </label>

            <label style={rowStyle}>
              <span>Font size</span>
              <input
                type="number" min={10} max={24} value={settings.fontSize}
                onChange={(e) => update({ fontSize: Math.min(24, Math.max(10, Number(e.target.value) || 12)) })}
                style={{ width: 52, fontSize: 11 }}
              />
            </label>

            <label style={rowStyle}>
              <span>Minimap</span>
              <input type="checkbox" checked={settings.minimap} onChange={(e) => update({ minimap: e.target.checked })} />
            </label>

            <label style={rowStyle}>
              <span>Vim mode</span>
              <input type="checkbox" checked={vimOn} onChange={(e) => { setVimMode(e.target.checked); setVimOn(e.target.checked); }} />
            </label>

            <label style={rowStyle}>
              <span>Line numbers</span>
              <select style={selectStyle} value={settings.lineNumbers} onChange={(e) => update({ lineNumbers: e.target.value as EditorSettingsLineNumbers })}>
                <option value="on">On</option>
                <option value="off">Off</option>
                <option value="relative">Relative</option>
              </select>
            </label>

            <label style={rowStyle}>
              <span>Tab size</span>
              <select style={selectStyle} value={settings.tabSize} onChange={(e) => update({ tabSize: Number(e.target.value) })}>
                <option value={2}>2</option>
                <option value={4}>4</option>
                <option value={8}>8</option>
              </select>
            </label>

            <label style={rowStyle}>
              <span>Whitespace</span>
              <select style={selectStyle} value={settings.renderWhitespace} onChange={(e) => update({ renderWhitespace: e.target.value as EditorSettingsWhitespace })}>
                <option value="none">None</option>
                <option value="boundary">Boundary</option>
                <option value="all">All</option>
              </select>
            </label>

            <div style={rowStyle}>
              <span>Diff layout</span>
              <div style={{ display: "flex", gap: 2 }}>
                <button style={segBtn(settings.diffSideBySide)} onClick={() => update({ diffSideBySide: true })}>Side</button>
                <button style={segBtn(!settings.diffSideBySide)} onClick={() => update({ diffSideBySide: false })}>Inline</button>
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  );
}

type EditorSettingsLineNumbers = "on" | "off" | "relative";
type EditorSettingsWhitespace = "none" | "boundary" | "all";
