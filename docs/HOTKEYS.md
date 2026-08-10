# Hotkeys

The in-app keyboard layer: a single frontend dispatcher parses key events into a
canonical **chord**, resolves it against a rebindable **keymap**, and dispatches
to the highest-priority **handler** registered for that command in the currently
active **scope**. It is purely in-webview — the OS-level `internal/hotkey`
(quick-attach) is separate and untouched.

## Layers

```
KeyboardEvent
  → registry.ts     declares every command once (id = object.action)
  → keymap.ts       merges registry defaults with DB `keybindings` overrides → chord ⇄ commandId
  → HotkeyProvider  one global keydown listener; leader-sequence state machine; scope stack; dispatch
  → scopes.ts       active-scope priority resolves same-chord collisions
  → useHotkey(...)  handlers subscribe in their component
  → WhichKeyHud     transient hint overlay for leader sequences
```

- **`hotkeys/registry.ts`** — the static `COMMANDS` array + `Command` type + label/ID helpers.
- **`hotkeys/chord.ts`** — pure `eventToToken` / `parseChord` / `formatChord`, canonical string round-trip. Heavily unit-tested.
- **`hotkeys/keymap.ts`** — resolves `mod` per platform, layers DB overrides over registry defaults into `chord → commandId` and `commandId → chord`.
- **`hotkeys/HotkeyProvider.tsx`** — the dispatcher context. Exposes `useHotkey`, `useHotkeyScope`, `useChordHint`, `useCommandList`.
- **`hotkeys/scopes.ts`** — the `Scope` constants and their priority order.
- **`hotkeys/WhichKeyHud.tsx`** — leader hint overlay.

## Command ID / label rule

- **Command ID = `object.action`** (e.g. `terminal.new`, `files.reload`, `search.global`). Stable and binding-independent — the chord is only a pointer to the ID.
- **Human label = `chord · Object · Action`** (e.g. `⌘T · Terminal · New`). The same three parts drive both the label and the help-modal (⌘H) filter.

## Scopes & priority

Highest priority wins when the same chord is bound in more than one active scope:

```
terminal  >  files  >  git  >  editor  >  global
```

A chord may map to **different** commands per scope, and that is encouraged.
Example: `⌘⇧]` = **terminal next** when the terminal is focused, and **switch
FILES → GIT** when the sidebar/editor is focused. Whichever scope is active on
top of the stack resolves it.

## Leader sequences (which-key)

Some objects act as a leader. Pressing `mod+<object>` (e.g. `⌘T`) opens a short
window and the **WhichKey HUD** shows the available follow keys
(`[N]ew [H]ere [I]nsert`). Press the action letter to complete
(`⌘T H` → `terminal.here`); if the window lapses, the object's **default
action** fires (`⌘T` → `terminal.new`). This is why `⌘T` is both a command and a
prefix — the default action is what the bare chord resolves to.

## Platform note

- `mod` resolves to **⌘ (Cmd) on darwin, Ctrl elsewhere**, via `runtime.EnvironmentInfo().platform`.
- Editor **zone-focus movement uses raw `Ctrl`+arrows on all platforms** (never `mod`) — Cmd+arrows collide with Monaco / input word-nav.

## How to add a new hotkey

1. Add a `Command` to `COMMANDS` in `hotkeys/registry.ts`: `id = object.action`, its `scope`, a `defaultChord`, an optional `sequence`, and (for a leader object) exactly one `defaultAction`.
2. Add a seed row to a **new** migration mirroring the command — store the modifier as neutral `cmd` / raw `ctrl`, never a resolved symbol.
3. In the owning component call `useHotkey("object.action", handler, { scope })`. If the handler only applies while its zone is focused, also call `useHotkeyScope(scope, isFocused)`.
4. (Optional) surface the chord in a button label via `useChordHint("object.action")`.
5. Verify: `go test ./internal/store/...` (migration must start with `-- +goose Up`, or app startup crashes) **and** the hotkey vitest suite (`cd frontend && npx vitest run src/hotkeys`).

Every binding is rebindable in **Settings → Keyboard Shortcuts** (sequence
capture + per-command Reset to default). Defaults ship via the seed migration.
