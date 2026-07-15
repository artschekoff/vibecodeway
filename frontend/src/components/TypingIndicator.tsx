interface TypingIndicatorProps {
  /** Text shown next to the dots. Defaults to a resource-fetch message. */
  label?: string;
}

/**
 * Telegram-style "typing" popup — three bouncing dots over a dimmed backdrop.
 * Used as a preloader while a context is created and its resources are fetched.
 */
export default function TypingIndicator({ label = "Fetching resources…" }: TypingIndicatorProps) {
  return (
    <div className="typing-indicator-overlay" role="status" aria-live="polite" aria-label={label}>
      <div className="typing-indicator-card">
        <span className="typing-indicator-dots" aria-hidden="true">
          <span className="typing-dot" />
          <span className="typing-dot" />
          <span className="typing-dot" />
        </span>
        <span className="typing-indicator-label">{label}</span>
      </div>
    </div>
  );
}
