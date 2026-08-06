import React from "react";
import { useHotkeyLeaderState } from "./HotkeyProvider";

const cap = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);

/**
 * Small bottom-center overlay shown while a leader sequence window is open.
 * Lists the pending object's sequenced actions, e.g.
 * "[H] Here  [I] Insert  ·  default: New"
 */
export function WhichKeyHud() {
  const { leader } = useHotkeyLeaderState();
  if (!leader) return null;

  const items = [...leader.children].sort((a, b) => (a.sequence ?? "").localeCompare(b.sequence ?? ""));

  return (
    <div className="which-key-hud" role="status" aria-live="polite">
      <span className="which-key-hud__object">{cap(leader.object)}</span>
      {items.map((b) => (
        <span className="which-key-hud__item" key={b.command.id}>
          <kbd>{(b.sequence ?? "").toUpperCase()}</kbd> {cap(b.command.action)}
        </span>
      ))}
      {leader.defaultBinding && (
        <span className="which-key-hud__default">default: {cap(leader.defaultBinding.command.action)}</span>
      )}
    </div>
  );
}
