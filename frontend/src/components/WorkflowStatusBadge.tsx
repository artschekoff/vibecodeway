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

// ponytail: inline hex darkening, upgrade to CSS color-mix() when browser support allows
function darken(hex: string, factor: number): string {
  const n = parseInt(hex.replace("#", ""), 16);
  const r = Math.round(((n >> 16) & 0xff) * factor).toString(16).padStart(2, "0");
  const g = Math.round(((n >> 8) & 0xff) * factor).toString(16).padStart(2, "0");
  const b = Math.round((n & 0xff) * factor).toString(16).padStart(2, "0");
  return `#${r}${g}${b}`;
}

export default function WorkflowStatusBadge({ status, isAI, title, statuses }: Props) {
  const match = statuses?.find((s) => s.slug === status);
  const label = match?.label ?? status.replace(/_/g, " ");
  const className = SYSTEM_CLASS[status] ?? "workflow-badge--custom";
  const color = match?.color;
  const style = color ? { color, backgroundColor: darken(color, 0.25) } : undefined;
  return (
    <span className={`workflow-badge ${className}${isAI ? " workflow-badge--ai" : ""}`} style={style} title={title}>
      {isAI && <span className="workflow-badge-ai-dot" title="AI proposed" />}
      {label}
    </span>
  );
}
