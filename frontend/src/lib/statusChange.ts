type SetStatusFn = (contextId: string, slug: string, comment: string) => Promise<unknown>;
type AddRelationFn = (
  contextId: string,
  targetId: string,
  relationType: string,
) => Promise<unknown>;

export interface StatusChangeInput {
  contextId: string;
  slug: string;
  comment: string;
  relationTargetId?: string;
  relationType?: string;
}

// Applies a workflow status change, then — only when a related context is
// chosen — records a typed relation from this context to the target.
// Sequential on purpose: a failed status change must not create a dangling relation.
export async function applyStatusChange(
  input: StatusChangeInput,
  setStatus: SetStatusFn,
  addRelation: AddRelationFn,
): Promise<void> {
  await setStatus(input.contextId, input.slug, input.comment);
  if (input.relationTargetId) {
    await addRelation(input.contextId, input.relationTargetId, input.relationType ?? "blocked-by");
  }
}
