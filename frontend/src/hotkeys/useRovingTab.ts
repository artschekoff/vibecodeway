import { useEffect, type RefObject } from "react";

const FOCUSABLE = "[tabindex],button,a,input,select";

/**
 * Keeps Tab / Shift+Tab focus inside `root`, moving in a visual order and
 * cycling at the ends. Only "ttb-rtl" (top→bottom, right→left) is used today.
 */
export function useRovingTab(root: RefObject<HTMLElement | null>, opts: { order: "ttb-rtl" }): void {
  const { order } = opts;
  useEffect(() => {
    const el = root.current;
    if (!el) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key !== "Tab") return;
      const items = orderFocusables(el);
      if (items.length === 0) return;
      e.preventDefault();
      const cur = document.activeElement as HTMLElement | null;
      const i = cur ? items.indexOf(cur) : -1;
      const dir = e.shiftKey ? -1 : 1;
      items[(i + dir + items.length) % items.length]?.focus();
    };
    el.addEventListener("keydown", handler);
    return () => el.removeEventListener("keydown", handler);
    // order is a literal today; listed for correctness if more orders are added.
  }, [root, order]);
}

function orderFocusables(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE))
    .filter((n) => !n.hasAttribute("disabled"))
    // ttb-rtl: primary top ascending, tiebreak right descending.
    .sort((a, b) => {
      const ra = a.getBoundingClientRect();
      const rb = b.getBoundingClientRect();
      return ra.top - rb.top || rb.right - ra.right;
    });
}
