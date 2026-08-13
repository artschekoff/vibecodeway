import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { eventToToken, formatChord, serializeChord, type Platform } from "./chord";
import { buildKeymap, type Binding } from "./keymap";
import { COMMANDS } from "./registry";
import { SCOPE_PRIORITY, type Scope } from "./scopes";
export type { Scope };
import type { Keybinding } from "../types";
import { WhichKeyHud } from "./WhichKeyHud";

/**
 * Dispatch algorithm (see task-0.4 brief):
 *  1. eventToToken(e, platform); ignore events that originate inside an
 *     <input>/<textarea>/[contenteditable] UNLESS the chord carries mod or
 *     ctrl (so ⌘F still works from an input, but a bare "t" doesn't hijack
 *     typing).
 *  2. If a leader is pending (previous key was a lead with sequenced
 *     children): treat this event's bare key as the sequence letter; find
 *     the matching child binding; if found, fire it and clear the leader;
 *     otherwise fire the leader's defaultAction (if any) and clear.
 *  3. Else look up lead.get(serialize(token)). If bindings exist:
 *       - If any have a `sequence` -> start a leader window
 *         (setTimeout(fireDefault, 600)), show the WhichKey HUD. Only
 *         preventDefault() the lead press if there IS a bare (non-sequence)
 *         command or a defaultAction bound directly to that lead chord -- a
 *         lead that has ONLY sequenced children and no bare/default command
 *         must NOT swallow the native key (so e.g. native ⌘C copy still
 *         works while ⌘C n is also bound as a leader).
 *       - Else (single bare binding, no sequenced siblings) fire it now.
 *  4. Firing = pick the highest-priority ACTIVE-scope handler registered
 *     for that command id (via SCOPE_PRIORITY); if one exists,
 *     preventDefault() + call it + emit `hotkey:<id>` on window. If no
 *     handler is registered/active, do nothing and let the key through.
 */

interface HandlerEntry {
  fn: () => void;
  scope: Scope;
  enabled: boolean;
}

export interface LeaderState {
  object: string;
  children: Binding[]; // sequenced children sharing this lead
  defaultBinding?: Binding; // the bare/default binding for this lead, if any
}

interface HotkeyContextValue {
  platform: Platform;
  keymapById: Map<string, Binding>;
  dispatchList: { command: (typeof COMMANDS)[number]; chord: string }[];
  leader: LeaderState | null;
  registry: Map<string, Set<HandlerEntry>>;
  activeScopes: Set<Scope>;
  runCommand: (commandId: string) => boolean;
  // Pause/resume keyboard dispatch. Used while capturing a new key combination
  // (rebinding) so the keys being pressed don't also trigger live shortcuts.
  suspendDispatch: (suspended: boolean) => void;
}

const HotkeyContext = createContext<HotkeyContextValue | null>(null);

const LEADER_TIMEOUT_MS = 600;

function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const tag = target.tagName;
  if (tag === "INPUT" || tag === "TEXTAREA") return true;
  return target.isContentEditable;
}

function fireHotkeyEvent(id: string) {
  window.dispatchEvent(new CustomEvent(`hotkey:${id}`));
}

const byScopePriority = (a: Binding, b: Binding) =>
  SCOPE_PRIORITY.indexOf(a.command.scope) - SCOPE_PRIORITY.indexOf(b.command.scope);

function orderByScopePriority(bindings: Binding[]): Binding[] {
  return [...bindings].sort(byScopePriority);
}

/**
 * A single physical chord can be the lead of bindings from unrelated
 * `command.object` groups (coincidental collision, e.g. mod+f is both the
 * bare "search.global" command and the lead of the sequenced
 * "files.focusSearch" leader). Resolve the raw lead bucket into the pieces
 * the dispatcher needs, considering only bindings that can ACTUALLY fire
 * right now (`canFire` = has a registered handler in a currently-active
 * scope). A binding that's merely registered but whose scope is inactive
 * (e.g. the files panel is mounted but not focused) does NOT count — that's
 * what lets ⌘F fire search.global immediately when files is inactive, yet
 * open a files leader when files is focused:
 *
 *   - `group`: the single object group that owns the sequenced children of
 *     THIS press — the live group with the most-specific active scope
 *     (SCOPE_PRIORITY). This is what a follow-key selects from.
 *   - `defaultBinding`: what the leader falls through to on timeout or on a
 *     non-matching follow-key. Searched across the WHOLE live bucket, not
 *     just `group`, so a bare binding on the same chord under a DIFFERENT
 *     object (⌘F -> search.global while files.focusSearch owns the leader)
 *     is retained as the fall-through instead of being discarded. Preference:
 *     a defaultAction/bare binding inside `group` first, else any other live
 *     bare binding on the lead (by SCOPE_PRIORITY).
 *
 * If nothing on the raw lead can fire, returns null so the caller does
 * nothing (no spurious leader / no swallowed native key).
 */
function resolveLead(
  rawBucket: Binding[],
  canFire: (b: Binding) => boolean
): { group: Binding[]; defaultBinding?: Binding } | null {
  const live = rawBucket.filter(canFire);
  if (live.length === 0) return null;

  // Group the live bindings by object; pick the owning group by
  // SCOPE_PRIORITY (most-specific active scope wins).
  const byObject = new Map<string, Binding[]>();
  for (const b of live) {
    const g = byObject.get(b.command.object);
    if (g) g.push(b);
    else byObject.set(b.command.object, [b]);
  }
  const groups = [...byObject.values()].sort((a, b) => byScopePriority(a[0], b[0]));
  const group = groups[0];

  // Default/fall-through binding: prefer a bare/defaultAction binding in the
  // owning group, else any other live bare binding on the same lead.
  const groupDefault = group.find((b) => b.command.defaultAction || !b.sequence);
  const crossDefault = orderByScopePriority(live.filter((b) => !b.sequence))[0];
  const defaultBinding = groupDefault ?? crossDefault;

  return { group, defaultBinding };
}

export function HotkeyProvider({
  platform,
  keybindings,
  children,
}: {
  platform: Platform;
  keybindings?: Keybinding[];
  children?: React.ReactNode;
}) {
  const keymap = useMemo(() => buildKeymap(COMMANDS, keybindings ?? []), [keybindings]);

  // commandId -> set of registered handlers (multiple scopes may register
  // the same command id; dispatch picks the most-specific active one).
  const registryRef = useRef<Map<string, Set<HandlerEntry>>>(new Map());
  // Currently active scopes (beyond "global", which is always active).
  const activeScopesRef = useRef<Set<Scope>>(new Set());

  // When true, the global keydown handler bails immediately — used while a
  // rebinding UI is capturing a combination.
  const suspendRef = useRef(false);
  const leaderTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const leaderPendingRef = useRef<{ children: Binding[]; defaultBinding?: Binding } | null>(null);
  const [leader, setLeader] = useState<LeaderState | null>(null);

  const isScopeActive = useCallback((scope: Scope) => scope === "global" || activeScopesRef.current.has(scope), []);

  const clearLeader = useCallback(() => {
    if (leaderTimerRef.current) {
      clearTimeout(leaderTimerRef.current);
      leaderTimerRef.current = null;
    }
    leaderPendingRef.current = null;
    setLeader(null);
  }, []);

  // The single enabled handler for `commandId` whose registered scope is
  // the most-specific currently-active one (per SCOPE_PRIORITY), or
  // undefined if none can fire.
  const pickHandler = useCallback(
    (commandId: string): HandlerEntry | undefined => {
      const handlers = registryRef.current.get(commandId);
      if (!handlers || handlers.size === 0) return undefined;
      for (const scope of SCOPE_PRIORITY) {
        if (!isScopeActive(scope)) continue;
        for (const h of handlers) {
          if (h.enabled && h.scope === scope) return h;
        }
      }
      return undefined;
    },
    [isScopeActive]
  );

  // A binding "can fire" iff it has a live handler in an active scope.
  const canFire = useCallback((b: Binding) => pickHandler(b.command.id) !== undefined, [pickHandler]);

  const fire = useCallback(
    (binding: Binding): boolean => {
      const chosen = pickHandler(binding.command.id);
      if (!chosen) return false;
      chosen.fn();
      fireHotkeyEvent(binding.command.id);
      return true;
    },
    [pickHandler]
  );

  // Explicitly run a command by id (e.g. selected from the help palette).
  // Unlike keyboard dispatch this is NOT gated on the handler's scope being
  // active: picking a command from a menu is an explicit request, so we fire
  // the active-scope handler if one exists, else the highest-priority enabled
  // handler regardless of focus. Returns false if nothing is registered.
  const runCommand = useCallback(
    (commandId: string): boolean => {
      let chosen = pickHandler(commandId);
      if (!chosen) {
        const handlers = registryRef.current.get(commandId);
        if (handlers) {
          for (const scope of SCOPE_PRIORITY) {
            for (const h of handlers) {
              if (h.enabled && h.scope === scope) { chosen = h; break; }
            }
            if (chosen) break;
          }
        }
      }
      if (!chosen) return false;
      chosen.fn();
      fireHotkeyEvent(commandId);
      return true;
    },
    [pickHandler]
  );

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (suspendRef.current) return; // a rebinding UI owns the keyboard
      const token = eventToToken(e, platform);
      if (!token) return;

      // Step 2: leader window pending — this key selects the sequence child.
      // Resolved BEFORE the typing-target guard: a pending leader means the user
      // already committed to a chord (e.g. ⌘T then H in a focused terminal), so
      // the plain follow-key must resolve it even though the target is "typing".
      if (leaderPendingRef.current) {
        const pending = leaderPendingRef.current;
        const child = pending.children.find((b) => b.sequence === token.key);
        clearLeader();
        if (child) {
          e.preventDefault();
          fire(child);
          return;
        }
        // Non-matching follow-key: fall through to the leader's resolved
        // default (e.g. ⌘F then a non-"f" key -> search.global). We do NOT
        // preventDefault here so the follow-key keeps its own native meaning
        // if the default fires or if nothing fires.
        if (pending.defaultBinding) fire(pending.defaultBinding);
        return;
      }

      const typing = isTypingTarget(e.target);
      if (typing && !token.mod && !token.ctrl) return;

      // Step 3: look up this chord as a potential lead. A single physical
      // chord can be shared by bindings from entirely unrelated objects
      // (e.g. mod+f is both the bare "search.global" command and the lead of
      // the sequenced "files.focusSearch" leader). `resolveLead` picks the
      // object group that owns the sequenced children of this press AND the
      // bare/default binding the leader falls through to on timeout / a
      // non-matching follow-key — which may be a bare binding from a
      // DIFFERENT object on the same chord (search.global), so global search
      // stays reachable while the files panel is focused. Returns null when
      // nothing on the lead is registered (no spurious leader, native key
      // preserved).
      const rawBucket = keymap.lead.get(serializeChord(token));
      if (!rawBucket || rawBucket.length === 0) return;
      const resolved = resolveLead(rawBucket, canFire);
      if (!resolved) return;

      const { group, defaultBinding } = resolved;
      const sequenced = group.filter((b) => b.sequence);

      if (sequenced.length > 0) {
        leaderPendingRef.current = { children: sequenced, defaultBinding };
        setLeader({
          object: group[0].command.object,
          children: sequenced,
          defaultBinding,
        });
        leaderTimerRef.current = setTimeout(() => {
          const pending = leaderPendingRef.current;
          clearLeader();
          if (pending?.defaultBinding) fire(pending.defaultBinding);
        }, LEADER_TIMEOUT_MS);

        // Only swallow the lead keypress if there IS a default/fall-through
        // command bound to this lead. A pure-sequence lead with no bare
        // default anywhere (e.g. ⌘C -> context.copyName "n", no default)
        // must let the native key through (native copy).
        if (defaultBinding) e.preventDefault();
        return;
      }

      // No sequenced children — try to fire a bare binding now. Multiple
      // unrelated bare commands can still share a chord across scopes (e.g.
      // mod+shift+] is both "sidebar.next" and "terminal.next"); try each in
      // SCOPE_PRIORITY order until one has an active registered handler.
      for (const binding of orderByScopePriority(group)) {
        if (fire(binding)) {
          e.preventDefault();
          return;
        }
      }
    };

    window.addEventListener("keydown", handler, true);
    return () => {
      window.removeEventListener("keydown", handler, true);
      if (leaderTimerRef.current) clearTimeout(leaderTimerRef.current);
    };
  }, [platform, keymap, fire, canFire, clearLeader]);

  const dispatchList = useMemo(
    () =>
      COMMANDS.map((command) => {
        const binding = keymap.byId.get(command.id);
        const chord = binding
          ? formatChord(binding.chord, platform) + (binding.sequence ? ` ${binding.sequence.toUpperCase()}` : "")
          : "";
        return { command, chord };
      }),
    [keymap, platform]
  );

  const value = useMemo<HotkeyContextValue>(
    () => ({
      platform,
      keymapById: keymap.byId,
      dispatchList,
      leader,
      registry: registryRef.current,
      activeScopes: activeScopesRef.current,
      runCommand,
      suspendDispatch: (suspended: boolean) => { suspendRef.current = suspended; },
    }),
    [platform, keymap, dispatchList, leader, runCommand]
  );

  return (
    <HotkeyContext.Provider value={value}>
      {children}
      <WhichKeyHud />
    </HotkeyContext.Provider>
  );
}

function useHotkeyContext(): HotkeyContextValue {
  const ctx = useContext(HotkeyContext);
  if (!ctx) throw new Error("Hotkey hooks must be used within a <HotkeyProvider>");
  return ctx;
}

/** Registers a handler for `commandId`. Cleans up on unmount. */
export function useHotkey(
  commandId: string,
  handler: () => void,
  opts?: { scope?: Scope; enabled?: boolean }
): void {
  const { registry } = useHotkeyContext();
  const scope = opts?.scope ?? "global";
  const enabled = opts?.enabled ?? true;

  // Keep the latest handler in a ref so re-renders don't need to
  // re-register (avoids churn while still calling the freshest closure).
  const handlerRef = useRef(handler);
  handlerRef.current = handler;

  useEffect(() => {
    const entry: HandlerEntry = {
      fn: () => handlerRef.current(),
      scope,
      enabled,
    };
    let set = registry.get(commandId);
    if (!set) {
      set = new Set();
      registry.set(commandId, set);
    }
    set.add(entry);
    return () => {
      set!.delete(entry);
      if (set!.size === 0) registry.delete(commandId);
    };
  }, [registry, commandId, scope, enabled]);
}

/** Pushes/pops a scope onto the active-scope set (e.g. terminal focus). */
export function useHotkeyScope(scope: Scope, active: boolean): void {
  const { activeScopes } = useHotkeyContext();
  useEffect(() => {
    if (!active) return;
    activeScopes.add(scope);
    return () => {
      activeScopes.delete(scope);
    };
  }, [activeScopes, scope, active]);
}

/** Live display chord for a command id (reflects rebinds). */
export function useChordHint(commandId: string): string {
  // Display-only: degrade to "" outside a provider so action buttons carrying a
  // <ChordHint> can render in trees (and tests) that don't mount HotkeyProvider.
  const ctx = useContext(HotkeyContext);
  if (!ctx) return "";
  const binding = ctx.keymapById.get(commandId);
  if (!binding) return "";
  return formatChord(binding.chord, ctx.platform) + (binding.sequence ? ` ${binding.sequence.toUpperCase()}` : "");
}

/** Full command list with resolved display chords, for the help modal. */
export function useCommandList(): { command: (typeof COMMANDS)[number]; chord: string }[] {
  const { dispatchList } = useHotkeyContext();
  return dispatchList;
}

/** Run a command by id (e.g. selected from the help palette). See runCommand. */
export function useCommandDispatch(): (commandId: string) => boolean {
  return useHotkeyContext().runCommand;
}

/**
 * Pause/resume global keyboard dispatch. Wrap a key-combination capture
 * (rebinding) in suspend(true)…suspend(false) so the pressed keys don't also
 * fire the shortcuts they're bound to.
 */
export function useSuspendHotkeys(): (suspended: boolean) => void {
  return useHotkeyContext().suspendDispatch;
}

/** Internal: exposes leader state + platform to WhichKeyHud. */
export function useHotkeyLeaderState(): { leader: LeaderState | null; platform: Platform } {
  const { leader, platform } = useHotkeyContext();
  return { leader, platform };
}
