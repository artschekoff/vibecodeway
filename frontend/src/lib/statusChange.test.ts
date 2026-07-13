import { describe, it, expect, vi } from "vitest";
import { applyStatusChange } from "./statusChange";

describe("applyStatusChange", () => {
  it("sets status and does NOT add a relation when no target", async () => {
    const setStatus = vi.fn().mockResolvedValue(undefined);
    const addRelation = vi.fn().mockResolvedValue(undefined);
    await applyStatusChange(
      { contextId: "c1", slug: "in_progress", comment: "go" },
      setStatus,
      addRelation,
    );
    expect(setStatus).toHaveBeenCalledWith("c1", "in_progress", "go");
    expect(addRelation).not.toHaveBeenCalled();
  });

  it("adds a relation with the default type when a target is given", async () => {
    const setStatus = vi.fn().mockResolvedValue(undefined);
    const addRelation = vi.fn().mockResolvedValue(undefined);
    await applyStatusChange(
      { contextId: "c1", slug: "needs_attention", comment: "blocked", relationTargetId: "c2" },
      setStatus,
      addRelation,
    );
    expect(setStatus).toHaveBeenCalledWith("c1", "needs_attention", "blocked");
    expect(addRelation).toHaveBeenCalledWith("c1", "c2", "blocked-by");
  });

  it("honors an explicit relationType", async () => {
    const setStatus = vi.fn().mockResolvedValue(undefined);
    const addRelation = vi.fn().mockResolvedValue(undefined);
    await applyStatusChange(
      { contextId: "c1", slug: "in_progress", comment: "", relationTargetId: "c2", relationType: "depends-on" },
      setStatus,
      addRelation,
    );
    expect(addRelation).toHaveBeenCalledWith("c1", "c2", "depends-on");
  });

  it("does not add a relation for an empty-string target", async () => {
    const setStatus = vi.fn().mockResolvedValue(undefined);
    const addRelation = vi.fn().mockResolvedValue(undefined);
    await applyStatusChange(
      { contextId: "c1", slug: "in_progress", comment: "", relationTargetId: "" },
      setStatus,
      addRelation,
    );
    expect(addRelation).not.toHaveBeenCalled();
  });

  it("does not attempt the relation when the status change fails", async () => {
    const setStatus = vi.fn().mockRejectedValue(new Error("boom"));
    const addRelation = vi.fn().mockResolvedValue(undefined);
    await expect(
      applyStatusChange(
        { contextId: "c1", slug: "in_progress", comment: "", relationTargetId: "c2" },
        setStatus,
        addRelation,
      ),
    ).rejects.toThrow("boom");
    expect(addRelation).not.toHaveBeenCalled();
  });
});
