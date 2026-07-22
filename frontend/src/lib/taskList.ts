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

export function moveTask(body: string, from: number, to: number): string {
  if (from === to) return body;
  const lines = parseBody(body);
  const taskPositions = lines.flatMap((l, i) => (l.kind === "task" ? [i] : []));
  const tasks = taskPositions.map((i) => lines[i]);
  const [moved] = tasks.splice(from, 1);
  tasks.splice(to, 0, moved);
  taskPositions.forEach((pos, k) => (lines[pos] = tasks[k]));
  return serialize(lines);
}
