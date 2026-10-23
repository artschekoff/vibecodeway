import { useEffect, type ChangeEvent, type TextareaHTMLAttributes } from "react";
import Editor from "@monaco-editor/react";
import { setupMonaco } from "../lib/monacoSetup";
import { useMonacoVim } from "../lib/useMonacoVim";
import { useVimMode } from "../lib/vimMode";

export type ProseEditorProps = TextareaHTMLAttributes<HTMLTextAreaElement> & {
  value: string;
  /**
   * Vim mode only: a bare Enter in NORMAL mode calls this instead of moving
   * the cursor — type, Esc, Enter to submit. INSERT keeps Enter as a newline;
   * the plain textarea keeps its call-site chord (⌘Enter) as the only submit.
   */
  onEnterSubmit?: () => void;
};

/**
 * A prose text box for the whole app. It takes exactly the props a `<textarea>`
 * takes — with vim mode off it *is* that textarea, forwarded untouched, so
 * behaviour and the global `textarea` styling are unchanged. With vim mode on it
 * swaps to Monaco in plaintext, sharing the vim layer with the code editor.
 */
export function ProseEditor({ onEnterSubmit, ...props }: ProseEditorProps) {
  const vim = useVimMode();
  if (!vim) return <textarea {...props} />;
  return <VimProseEditor {...props} onEnterSubmit={onEnterSubmit} />;
}

const LINE_HEIGHT = 20;

function VimProseEditor({ value, onChange, onKeyDown, onFocus, onBlur, onEnterSubmit, autoFocus, rows, style, disabled, readOnly, "aria-label": ariaLabel }: ProseEditorProps) {
  const { attach, VimStatusBar, getVimMode } = useMonacoVim({ startInInsert: true });

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
      // Focus/blur bubble up from Monaco's hidden textarea; call sites use them
      // to show helpers (e.g. JQL template chips) exactly as with a textarea.
      onFocus={(e) => onFocus?.(e as unknown as React.FocusEvent<HTMLTextAreaElement>)}
      onBlur={(e) => onBlur?.(e as unknown as React.FocusEvent<HTMLTextAreaElement>)}
      // Capture phase, so submit chords like ⌘Enter reach the call site's
      // onKeyDown (textarea parity) before Monaco/vim swallow the keystroke.
      onKeyDownCapture={(e) => {
        onKeyDown?.(e as unknown as React.KeyboardEvent<HTMLTextAreaElement>);
        if (!e.defaultPrevented && onEnterSubmit && e.key === "Enter"
          && !e.metaKey && !e.ctrlKey && !e.altKey && !e.shiftKey && getVimMode() === "normal") {
          e.preventDefault();
          onEnterSubmit();
        }
        // Handler consumed the chord (e.g. submitted on ⌘Enter) — keep it away
        // from Monaco/vim entirely.
        if (e.defaultPrevented) e.stopPropagation();
      }}
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
          // Breathing room: Monaco's padding option is vertical-only, so the
          // left inset comes from the (empty) decorations gutter.
          lineDecorationsWidth: 10,
          lineNumbersMinChars: 0,
          overviewRulerLanes: 0,
          scrollbar: { vertical: "auto", horizontal: "hidden" },
          scrollBeyondLastLine: false,
          renderLineHighlight: "none",
          fontFamily: 'Verdana, Geneva, "DejaVu Sans", sans-serif',
          fontSize: 13,
          lineHeight: LINE_HEIGHT,
          padding: { top: 10, bottom: 10 },
        }}
        onMount={(editor) => {
          attach(editor);
          // textarea parity: the caret lands inside straight away, so a click
          // on "add note" is followed by typing, not by a second click.
          if (autoFocus) editor.focus();
        }}
      />
      <VimStatusBar />
    </div>
  );
}
