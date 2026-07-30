import { describe, it, expect } from "vitest";
import { buildFileTree } from "./fileTree";

describe("buildFileTree", () => {
  it("puts directories before files, both alphabetical", () => {
    const t = buildFileTree(["z.ts", "src/b.ts", "a.ts"]);
    expect(t.map((n) => `${n.type}:${n.name}`)).toEqual(["dir:src", "file:a.ts", "file:z.ts"]);
    const src = t[0];
    expect(src.type === "dir" && src.children).toEqual([{ type: "file", name: "b.ts", path: "src/b.ts" }]);
  });

  it("keeps the original full path on file leaves", () => {
    const t = buildFileTree(["src/deep/x.ts"]);
    // compacted to one dir node "src/deep" with the file inside
    expect(t).toEqual([
      { type: "dir", name: "src/deep", path: "src/deep", children: [{ type: "file", name: "x.ts", path: "src/deep/x.ts" }] },
    ]);
  });

  it("compacts single-child folder chains (VSCode style)", () => {
    const t = buildFileTree(["src/functions/cf/index.ts", "src/functions/cf/util.ts"]);
    expect(t).toHaveLength(1);
    expect(t[0].type === "dir" && t[0].name).toBe("src/functions/cf");
    expect(t[0].type === "dir" && t[0].children.map((c) => c.name)).toEqual(["index.ts", "util.ts"]);
  });

  it("does NOT compact a folder that branches", () => {
    const t = buildFileTree(["src/a/x.ts", "src/b/y.ts"]);
    expect(t).toHaveLength(1);
    const src = t[0];
    expect(src.type === "dir" && src.name).toBe("src");
    expect(src.type === "dir" && src.children.map((c) => c.name)).toEqual(["a", "b"]);
  });

  it("can disable compaction", () => {
    const t = buildFileTree(["src/deep/x.ts"], false);
    expect(t[0].type === "dir" && t[0].name).toBe("src");
  });
});
