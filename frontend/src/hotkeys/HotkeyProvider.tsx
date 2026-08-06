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

function orderByScopePriority(bindings: Binding[]): Binding[] {
  return [...bindings].sort(
    (a, b) => SCOPE_PRIORITY.indexOf(a.command.scope) - SCOPE_PRIORITY.indexOf(b.command.scope)
  );
}

/**
 * A single chord can be the lead of bindings from unrelated `command.object`
 * groups (coincidental collision, e.g. mod+f = bare "search.global" AND the
 * lead of sequenced "files.focusSearch"). Narrow to the one object group
 * that's actually live: prefer the group(s) with a currently-registered
 * handler, breaking ties by SCOPE_PRIORITY; fall back to the full raw
 * bucket if nothing is registered yet.
 */
function pickActiveObjectGroup(bucket: Binding[], registry: Map<string, Set<unknown>>): Binding[] {
  const byObject = new Map<string, Binding[]>();
  for (const b of bucket) {
    const group = byObject.get(b.command.object);
    if (group) group.push(b);
    else byObject.set(b.command.object, [b]);
  }
  if (byObject.size <= 1) return bucket;

  const registeredGroups = [...byObject.values()].filter((group) =>
    group.some((b) => (registry.get(b.command.id)?.size ?? 0) > 0)
  );
  if (registeredGroups.length === 0) return bucket;
  if (registeredGroups.length === 1) return registeredGroups[0];

  registeredGroups.sort(
    (a, b) => SCOPE_PRIORITY.indexOf(a[0].command.scope) - SCOPE_PRIORITY.indexOf(b[0].command.scope)
  );
  return registeredGroups[0];
}

export function HotkeyProvider({
  platform,
  keybindings,
  children,
}: {
  platform: Platform;
  keybindings?: Keybinding[];
  children: React.ReactNode;
}) {
  const keymap = useMemo(() => buildKeymap(COMMANDS, keybindings ?? []), [keybindings]);

  // commandId -> set of registered handlers (multiple scopes may register
  // the same command id; dispatch picks the most-specific active one).
  const registryRef = useRef<Map<string, Set<HandlerEntry>>>(new Map());
  // Currently active scopes (beyond "global", which is always active).
  const activeScopesRef = useRef<Set<Scope>>(new Set());

  const leaderTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const leaderBindingsRef = useRef<Binding[] | null>(null);
  const [leader, setLeader] = useState<LeaderState | null>(null);

  const isScopeActive = useCallback((scope: Scope) => scope === "global" || activeScopesRef.current.has(scope), []);

  const clearLeader = useCallback(() => {
    if (leaderTimerRef.current) {
      clearTimeout(leaderTimerRef.current);
      leaderTimerRef.current = null;
    }
    leaderBindingsRef.current = null;
    setLeader(null);
  }, []);

  const fire = useCallback(
    (binding: Binding): boolean => {
      const handlers = registryRef.current.get(binding.command.id);
      if (!handlers || handlers.size === 0) return false;

      let chosen: HandlerEntry | undefined;
      for (const scope of SCOPE_PRIORITY) {
        if (!isScopeActive(scope)) continue;
        for (const h of handlers) {
          if (h.enabled && h.scope === scope) {
            chosen = h;
            break;
          }
        }
        if (chosen) break;
      }
      if (!chosen) return false;

      chosen.fn();
      fireHotkeyEvent(binding.command.id);
      return true;
    },
    [isScopeActive]
  );

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const token = eventToToken(e, platform);
      if (!token) return;

      const typing = isTypingTarget(e.target);
      if (typing && !token.mod && !token.ctrl) return;

      // Step 2: leader window pending — this key selects the sequence child.
      if (leaderBindingsRef.current) {
        const pending = leaderBindingsRef.current;
        const child = pending.find((b) => b.sequence === token.key);
        clearLeader();
        if (child) {
          e.preventDefault();
          fire(child);
          return;
        }
        const defaultBinding = pending.find((b) => b.command.defaultAction);
        if (defaultBinding) {
          fire(defaultBinding);
        }
        return;
      }

      // Step 3: look up this chord as a potential lead. A single physical
      // chord can be shared by bindings from entirely unrelated objects
      // (e.g. mod+f is both the bare "search.global" command and the lead
      // of the sequenced "files.focusSearch" leader) — these are different
      // features that just happen to collide on the same chord, and only
      // one is ever relevant at a time. Narrow the raw bucket down to the
      // single `command.object` group that's actually in play before
      // deciding whether this press starts a leader window or fires
      // immediately:
      //   - Prefer the object group that has at least one handler
      //     currently registered (via useHotkey) — that's the group whose
      //     feature is actually mounted/active right now.
      //   - If more than one object group has a registered handler, break
      //     the tie using SCOPE_PRIORITY (most-specific active scope
      //     wins).
      //   - If no group has a registered handler, keep the whole raw
      //     bucket so any bare/default fire attempt below still no-ops
      //     safely.
      const rawBucket = keymap.lead.get(serializeChord(token));
      if (!rawBucket || rawBucket.length === 0) return;
      const bucket = pickActiveObjectGroup(rawBucket, registryRef.current);

      const sequenced = bucket.filter((b) => b.sequence);
      const bareOrDefault = bucket.find((b) => !b.sequence || b.command.defaultAction);

      if (sequenced.length > 0) {
        leaderBindingsRef.current = bucket;
        setLeader({
          object: bucket[0].command.object,
          children: sequenced,
          defaultBinding: bareOrDefault,
        });
        leaderTimerRef.current = setTimeout(() => {
          const pending = leaderBindingsRef.current;
          clearLeader();
          if (pending) {
            const defaultBinding = pending.find((b) => b.command.defaultAction);
            if (defaultBinding) fire(defaultBinding);
          }
        }, LEADER_TIMEOUT_MS);

        // Only swallow the lead keypress if it also has a bare/default
        // command bound directly to it. A lead with ONLY sequenced children
        // must let the native key through (e.g. native ⌘C copy).
        if (bareOrDefault) e.preventDefault();
        return;
      }

      // No sequenced siblings in the chosen group — try to fire a bare
      // binding now. Multiple unrelated bare commands can still share a
      // chord across scopes (e.g. mod+shift+] is both "sidebar.next" and
      // "terminal.next"); try each in SCOPE_PRIORITY order until one has
      // an active registered handler.
      for (const binding of orderByScopePriority(bucket)) {
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
  }, [platform, keymap, fire, clearLeader, isScopeActive]);

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
    }),
    [platform, keymap, dispatchList, leader]
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
  const { keymapById, platform } = useHotkeyContext();
  const binding = keymapById.get(commandId);
  if (!binding) return "";
  return formatChord(binding.chord, platform) + (binding.sequence ? ` ${binding.sequence.toUpperCase()}` : "");
}

/** Full command list with resolved display chords, for the help modal. */
export function useCommandList(): { command: (typeof COMMANDS)[number]; chord: string }[] {
  const { dispatchList } = useHotkeyContext();
  return dispatchList;
}

/** Internal: exposes leader state + platform to WhichKeyHud. */
export function useHotkeyLeaderState(): { leader: LeaderState | null; platform: Platform } {
  const { leader, platform } = useHotkeyContext();
  return { leader, platform };
}
