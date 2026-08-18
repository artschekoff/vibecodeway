import { describe, it, expect } from "vitest";
import { buildKeymap } from "./keymap";
import { COMMANDS } from "./registry";
import { serializeChord, parseChord } from "./chord";

describe("buildKeymap", () => {
  it("groups bare + sequenced bindings under the same lead", () => {
    const km = buildKeymap(COMMANDS, []); // no overrides -> defaults
    // ⌘E is the canonical shared lead: bare editor.open plus the four zone-focus
    // sequence children. (⌘T deliberately is NOT a lead — see registry.ts.)
    const lead = km.lead.get(serializeChord(parseChord("mod+e")));
    expect(lead?.map((b) => b.command.id).sort())
      .toEqual(["editor.focusDown", "editor.focusLeft", "editor.focusRight", "editor.focusUp", "editor.open"]);
  });

  it("keeps ⌘T a lone bare binding so a new terminal opens on the keypress", () => {
    const km = buildKeymap(COMMANDS, []);
    const lead = km.lead.get(serializeChord(parseChord("mod+t")));
    expect(lead?.map((b) => b.command.id)).toEqual(["terminal.new"]);
    expect(lead?.[0].sequence).toBeUndefined();
  });
  it("applies DB overrides over defaults", () => {
    const km = buildKeymap(COMMANDS, [
      { command: "search.global", modifierKeys: "cmd,shift", keyCode: "k", sequence: "" } as any,
    ]);
    expect(km.byId.get("search.global")!.chord).toEqual(parseChord("mod+shift+k"));
  });
});
