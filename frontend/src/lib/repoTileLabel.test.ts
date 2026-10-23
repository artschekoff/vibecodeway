import { describe, it, expect } from "vitest";
import { repoTileLabel } from "./repoTileLabel";

describe("repoTileLabel", () => {
  it("uses the last ticket id from the branch", () => {
    expect(repoTileLabel("acme/acme-functions", "feat_JH-5612_task_JH-5638_acme-functions-development"))
      .toEqual({ top: "acme-functions", sub: "JH-5638" });
  });
  it("falls back to the raw branch when no ticket id", () => {
    expect(repoTileLabel("rdbms", "main")).toEqual({ top: "rdbms", sub: "main" });
  });
  it("handles missing branch", () => {
    expect(repoTileLabel("acme/rdbms")).toEqual({ top: "rdbms", sub: "" });
  });
});
