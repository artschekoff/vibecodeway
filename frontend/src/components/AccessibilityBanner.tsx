import { useState, useCallback } from "react";
import { OpenAccessibilityPreferences } from "../../wailsjs/go/main/App";

interface Props {
  onDismiss: () => void;
}

export const AccessibilityBanner = ({ onDismiss }: Props) => {
  const [opening, setOpening] = useState(false);

  const handleOpen = useCallback(() => {
    setOpening(true);
    OpenAccessibilityPreferences()
      .catch(() => {})
      .finally(() => setOpening(false));
  }, []);

  return (
    <div className="a11y-banner">
      <span className="a11y-banner-icon">⌨️</span>
      <div className="a11y-banner-body">
        <strong>Accessibility permission needed</strong>
        <span>
          Grant access so{" "}
          <kbd>⌘ Shift C</kbd> can capture selected text from other apps.
        </span>
      </div>
      <button className="btn-secondary a11y-banner-btn" onClick={handleOpen} disabled={opening}>
        {opening ? "Opening…" : "Open System Settings"}
      </button>
      <button className="a11y-banner-close" onClick={onDismiss} title="Dismiss">
        ✕
      </button>
    </div>
  );
};

