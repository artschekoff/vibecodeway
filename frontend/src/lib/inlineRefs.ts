export function splitInlineCode(text: string): Array<{ code: boolean; value: string }> {
  const out: Array<{ code: boolean; value: string }> = [];
  const re = /`([^`]+)`/g;
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text))) {
    if (m.index > last) out.push({ code: false, value: text.slice(last, m.index) });
    out.push({ code: true, value: m[1] });
    last = m.index + m[0].length;
  }
  if (last < text.length) out.push({ code: false, value: text.slice(last) });
  return out.length ? out : [{ code: false, value: text }];
}
