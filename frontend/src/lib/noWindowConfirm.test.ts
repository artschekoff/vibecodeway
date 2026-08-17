import { describe, it, expect } from "vitest";

// window.confirm() is a no-op in the Wails WKWebView: it returns false without
// ever prompting, so every confirm-gated action silently does nothing. Use the
// ConfirmAction Go binding (native dialog) instead.
// ponytail: source scan, not a render test — the regression is textual.
const sources = import.meta.glob("../**/*.{ts,tsx}", { query: "?raw", import: "default", eager: true }) as Record<string, string>;

describe("window.confirm", () => {
  it("is never used in frontend sources", () => {
    const offenders = Object.entries(sources)
      .filter(([path]) => !path.includes("noWindowConfirm.test"))
      .filter(([, src]) =>
        src.split("\n").some((l) => !l.trim().startsWith("//") && /window\.confirm\s*\(/.test(l)),
      )
      .map(([path]) => path);
    expect(offenders).toEqual([]);
  });
});
