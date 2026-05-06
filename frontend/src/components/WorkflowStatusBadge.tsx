import type { WorkflowStatus } from "../types";

interface Props {
  status: string;
  isAI?: boolean;  // true = badge comes from AI proposal, false = user override
  title?: string;
  statuses?: WorkflowStatus[];  // live list from DB; resolves labels for custom statuses
}

const SYSTEM_CLASS: Record<string, string> = {
  not_started:     "workflow-badge--not-started",
  scoping:         "workflow-badge--scoping",
  in_progress:     "workflow-badge--in-progress",
  needs_attention: "workflow-badge--needs-attention",
  ready:           "workflow-badge--ready",
};

export default function WorkflowStatusBadge({ status, isAI, title, statuses }: Props) {
  const dynamicLabel = statuses?.find((s) => s.slug === status)?.label;
  const label = dynamicLabel ?? status.replace(/_/g, " ");
  const className = SYSTEM_CLASS[status] ?? "workflow-badge--custom";
  return (
    <span className={`workflow-badge ${className}${isAI ? " workflow-badge--ai" : ""}`} title={title}>
      {isAI && <span className="workflow-badge-ai-dot" title="AI proposed" />}
      {label}
    </span>
  );
}
