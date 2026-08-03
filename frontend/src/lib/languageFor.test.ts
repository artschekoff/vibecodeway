import { describe, it, expect, beforeEach } from "vitest";
import { languageFor, setLanguageRegistry, resetLanguageRegistry } from "./languageFor";

// Module-level registry state persists between tests in one file, so we reset
// it in beforeEach and use resetLanguageRegistry() for the fallback cases
// rather than vi.resetModules() + dynamic import (simpler, no async).
describe("languageFor", () => {
  beforeEach(() => resetLanguageRegistry());

  describe("with an injected registry", () => {
    beforeEach(() => {
      setLanguageRegistry([
        { id: "javascript", extensions: [".mjs", ".cjs"] },
        { id: "typescript", extensions: [".mts"] },
        { id: "dockerfile", filenames: ["Dockerfile"] },
      ]);
    });

    it("resolves .mjs to javascript", () => {
      expect(languageFor("foo.mjs")).toBe("javascript");
    });

    it("resolves .mts to typescript", () => {
      expect(languageFor("foo.mts")).toBe("typescript");
    });

    it("resolves Dockerfile by filename (case-insensitive)", () => {
      expect(languageFor("Dockerfile")).toBe("dockerfile");
      expect(languageFor("path/to/dockerfile")).toBe("dockerfile");
    });

    it("resolves FOO.MJS case-insensitively", () => {
      expect(languageFor("FOO.MJS")).toBe("javascript");
    });

    it("returns plaintext for an unknown extension", () => {
      expect(languageFor("x.zzz")).toBe("plaintext");
    });
  });

  describe("fallback (no registry injected)", () => {
    it("resolves .mjs to javascript", () => {
      expect(languageFor("foo.mjs")).toBe("javascript");
    });

    it("resolves .ts to typescript", () => {
      expect(languageFor("foo.ts")).toBe("typescript");
    });

    it("returns plaintext for an unknown extension", () => {
      expect(languageFor("foo.zzz")).toBe("plaintext");
    });
  });
});
