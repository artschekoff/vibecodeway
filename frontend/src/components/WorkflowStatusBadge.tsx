interface Props {
  status: string;
  isAI?: boolean;  // true = badge comes from AI proposal, false = user override
  title?: string;
}

const STATUS_CONFIG: Record<string, { label: string; className: string }> = {
  not_started:     { label: "Not started",    className: "workflow-badge--not-started" },
  scoping:         { label: "Scoping",         className: "workflow-badge--scoping" },
  in_progress:     { label: "In progress",     className: "workflow-badge--in-progress" },
  needs_attention: { label: "Needs attention", className: "workflow-badge--needs-attention" },
  ready:           { label: "Ready",           className: "workflow-badge--ready" },
};

export default function WorkflowStatusBadge({ status, isAI, title }: Props) {
  const cfg = STATUS_CONFIG[status] ?? { label: status, className: "workflow-badge--unknown" };
  return (
    <span className={`workflow-badge ${cfg.className}${isAI ? " workflow-badge--ai" : ""}`} title={title}>
      {isAI && <span className="workflow-badge-ai-dot" title="AI proposed" />}
      {cfg.label}
    </span>
  );
}
