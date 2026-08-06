import { describe, it, expect } from "vitest";
import { buildKeymap } from "./keymap";
import { COMMANDS } from "./registry";
import { serializeChord, parseChord } from "./chord";

describe("buildKeymap", () => {
  it("groups bare + sequenced bindings under the same lead", () => {
    const km = buildKeymap(COMMANDS, []); // no overrides -> defaults
    const lead = km.lead.get(serializeChord(parseChord("mod+t")));
    expect(lead?.map((b) => b.command.id).sort())
      .toEqual(["terminal.here", "terminal.insert", "terminal.new"]);
  });
  it("applies DB overrides over defaults", () => {
    const km = buildKeymap(COMMANDS, [
      { command: "search.global", modifierKeys: "cmd,shift", keyCode: "k", sequence: "" } as any,
    ]);
    expect(km.byId.get("search.global")!.chord).toEqual(parseChord("mod+shift+k"));
  });
});
