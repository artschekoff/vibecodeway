import { useEffect, type ChangeEvent, type TextareaHTMLAttributes } from "react";
import Editor from "@monaco-editor/react";
import { setupMonaco } from "../lib/monacoSetup";
import { useMonacoVim } from "../lib/useMonacoVim";
import { useVimMode } from "../lib/vimMode";

export type ProseEditorProps = TextareaHTMLAttributes<HTMLTextAreaElement> & { value: string };

/**
 * A prose text box for the whole app. It takes exactly the props a `<textarea>`
 * takes — with vim mode off it *is* that textarea, forwarded untouched, so
 * behaviour and the global `textarea` styling are unchanged. With vim mode on it
 * swaps to Monaco in plaintext, sharing the vim layer with the code editor.
 */
export function ProseEditor(props: ProseEditorProps) {
  const vim = useVimMode();
  if (!vim) return <textarea {...props} />;
  return <VimProseEditor {...props} />;
}

const LINE_HEIGHT = 20;

function VimProseEditor({ value, onChange, rows, style, disabled, readOnly, "aria-label": ariaLabel }: ProseEditorProps) {
  const { attach, VimStatusBar } = useMonacoVim();

  useEffect(() => { setupMonaco(); }, []);

  // Monaco needs a pixel height; mirror what `rows` would have given a textarea,
  // and honour an explicit height/minHeight from the call site.
  const explicit = typeof style?.height === "number" ? style.height
    : typeof style?.minHeight === "number" ? style.minHeight
    : undefined;
  const height = explicit ?? (typeof rows === "number" ? rows * LINE_HEIGHT + 16 : 120);

  // Call sites keep their textarea onChange handlers, so hand them the only
  // field those handlers read. ponytail: a full synthetic event would be
  // ceremony — every handler in the app reads e.target.value and nothing else.
  const emit = (next: string) => {
    onChange?.({ target: { value: next } } as ChangeEvent<HTMLTextAreaElement>);
  };

  return (
    <div
      aria-label={ariaLabel}
      style={{ border: "1px solid var(--border)", borderRadius: "var(--radius-sm)", overflow: "hidden", background: "var(--surface2)" }}
    >
      <Editor
        height={height}
        language="plaintext"
        value={value}
        onChange={(next) => emit(next ?? "")}
        options={{
          readOnly: disabled || readOnly,
          // Prose, not code: no gutter furniture, wrap long lines, and the same
          // Verdana the plain textareas use so toggling vim doesn't reflow text.
          lineNumbers: "off",
          minimap: { enabled: false },
          wordWrap: "on",
          folding: false,
          glyphMargin: false,
          lineDecorationsWidth: 0,
          lineNumbersMinChars: 0,
          overviewRulerLanes: 0,
          scrollbar: { vertical: "auto", horizontal: "hidden" },
          scrollBeyondLastLine: false,
          renderLineHighlight: "none",
          fontFamily: 'Verdana, Geneva, "DejaVu Sans", sans-serif',
          fontSize: 13,
          lineHeight: LINE_HEIGHT,
          padding: { top: 8, bottom: 8 },
        }}
        onMount={(editor) => attach(editor)}
      />
      <VimStatusBar />
    </div>
  );
}
