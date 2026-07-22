export type TaskLine =
  | { kind: "task"; checked: boolean; text: string }
  | { kind: "text"; raw: string };

const TASK_RE = /^- \[( |x)\] ?(.*)$/;

export function parseBody(body: string): TaskLine[] {
  return body.split("\n").map((raw) => {
    const m = raw.match(TASK_RE);
    if (m) return { kind: "task", checked: m[1] === "x", text: m[2] };
    return { kind: "text", raw };
  });
}

export function serialize(lines: TaskLine[]): string {
  return lines
    .map((l) => (l.kind === "task" ? `- [${l.checked ? "x" : " "}] ${l.text}` : l.raw))
    .join("\n");
}

export function toggleAt(body: string, taskIndex: number): string {
  const lines = parseBody(body);
  let n = -1;
  for (const l of lines) {
    if (l.kind === "task" && ++n === taskIndex) l.checked = !l.checked;
  }
  return serialize(lines);
}

export function addStep(body: string, text = ""): string {
  return body === "" ? `- [ ] ${text}` : `${body}\n- [ ] ${text}`;
}
