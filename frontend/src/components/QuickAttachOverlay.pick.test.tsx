import { describe, it, expect, vi } from "vitest";

vi.mock("../../wailsjs/go/main/App", () => ({}));

import { pickAttachTargets } from "./QuickAttachOverlay";
import type { Context, Organization } from "../types";

const ctx = (id: string, orgId: string, status = "active") =>
  ({ id, key: id, orgId, status }) as unknown as Context;
const orgs = [
  { id: "o1", name: "acme" },
  { id: "orev", name: "Code Reviews" },
] as unknown as Organization[];

describe("pickAttachTargets", () => {
  const all = [ctx("a", "o1"), ctx("b", "orev"), ctx("c", "o1", "archived")];

  it("with an org: only that org's active contexts", () => {
    expect(pickAttachTargets(all, orgs, "o1").map((c) => c.id)).toEqual(["a"]);
  });

  it("without an org: all active contexts except the Code Reviews org", () => {
    expect(pickAttachTargets(all, orgs).map((c) => c.id)).toEqual(["a"]);
  });

  it("explicitly inside Code Reviews still lists its contexts", () => {
    expect(pickAttachTargets(all, orgs, "orev").map((c) => c.id)).toEqual(["b"]);
  });
});
